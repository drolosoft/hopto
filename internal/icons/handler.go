package icons

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// RoutePrefix is where the page asks for icons: /user-icons/<id>.png.
const RoutePrefix = "/user-icons/"

// idPattern is the shape of every id, the same as the library's; anything
// else in the path is refused before touching the file system.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// cacheEntries bounds the decoded icons kept in memory; past it the cache
// is simply dropped, which is cheaper than an eviction policy for a few
// hundred small images.
const cacheEntries = 512

// Handler serves user icons from the icons folder and bundle icons from
// their .icns, every one of them re-encoded by Normalize. It is mounted as
// the fallback handler of the Wails asset server.
type Handler struct {
	root    *os.Root
	resolve func(id string) (string, bool)

	mu    sync.Mutex
	cache map[string]cachedIcon
}

// cachedIcon is a decoded icon and the stamp of the file it came from.
type cachedIcon struct {
	stamp string
	png   []byte
}

// NewHandler opens dir (creating it, private to the user) and keeps the
// resolver that maps a discovered app id to its .icns path.
func NewHandler(
	dir string, resolve func(id string) (string, bool),
) (*Handler, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}

	handler := &Handler{
		root:    root,
		resolve: resolve,
		cache:   map[string]cachedIcon{},
	}

	return handler, nil
}

// ServeHTTP answers GET /user-icons/<id>.png.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, ok := idFromPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	icon, ok := h.lookup(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// The URL carries the file's stamp, so the bytes behind one URL never
	// change and the web view may keep them for good.
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Length", strconv.Itoa(len(icon)))
	_, _ = w.Write(icon)
}

// idFromPath extracts and validates the id of a request path.
func idFromPath(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, RoutePrefix)
	if !ok {
		return "", false
	}

	id, ok := strings.CutSuffix(rest, ".png")
	if !ok || !idPattern.MatchString(id) {
		return "", false
	}

	return id, true
}

// lookup finds the icon of an id: the user's own file first, then the
// bundle the resolver names. Both go through the cache keyed by the file's
// stamp, so a replaced file is decoded again and an untouched one is not.
func (h *Handler) lookup(id string) ([]byte, bool) {
	info, statErr := h.root.Stat(id + ".png")
	if statErr == nil && info.Mode().IsRegular() {
		stamp := fmt.Sprintf(
			"file:%d:%d", info.ModTime().UnixNano(), info.Size(),
		)

		return h.cached(id, stamp, func() ([]byte, error) {
			file, err := h.root.Open(id + ".png")
			if err != nil {
				return nil, err
			}
			defer func() {
				// Best effort: the file was read whole already.
				_ = file.Close()
			}()

			data, err := io.ReadAll(file)
			if err != nil {
				return nil, err
			}

			return Normalize(data, AppSide)
		})
	}

	if h.resolve == nil {
		return nil, false
	}

	icnsPath, ok := h.resolve(id)
	if !ok {
		return nil, false
	}

	info, err := os.Stat(icnsPath)
	if err != nil {
		return nil, false
	}

	stamp := fmt.Sprintf("icns:%s:%d", icnsPath, info.ModTime().UnixNano())

	return h.cached(id, stamp, func() ([]byte, error) {
		raw, ok := PNG(icnsPath)
		if !ok {
			return nil, errors.New("icons: no PNG in " + icnsPath)
		}

		return Normalize(raw, AppSide)
	})
}

// cached returns the icon for id when its stamp is unchanged, or loads it.
func (h *Handler) cached(
	id, stamp string, load func() ([]byte, error),
) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry, ok := h.cache[id]; ok && entry.stamp == stamp {
		return entry.png, true
	}

	icon, err := load()
	if err != nil {
		return nil, false
	}

	if len(h.cache) >= cacheEntries {
		h.cache = map[string]cachedIcon{}
	}

	h.cache[id] = cachedIcon{stamp: stamp, png: icon}

	return icon, true
}

// FileURL is the page URL of a user icon, with the file's modification
// time as a cache buster, or "" when there is no such file.
func FileURL(dir, id string) string {
	if !idPattern.MatchString(id) {
		return ""
	}

	info, err := os.Stat(filepath.Join(dir, id+".png"))
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}

	return StampURL(id, info.ModTime().Unix())
}

// StampURL builds the page URL of an icon from its id and a stamp (a
// modification time); the handler ignores the query, the web view uses it
// to tell versions apart.
func StampURL(id string, stamp int64) string {
	return RoutePrefix + id + ".png?v=" + strconv.FormatInt(stamp, 10)
}
