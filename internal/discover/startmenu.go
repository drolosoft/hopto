package discover

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/drolosoft/hopto/internal/library"
)

// Windows apps are the shortcuts of the two Start Menus. The shortcut is
// what opens (it carries the arguments and the working folder); its
// target only tells hopto what kind of thing it is.

// documentExtensions are targets that are not programs.
var documentExtensions = map[string]bool{
	".url": true, ".txt": true, ".pdf": true, ".htm": true,
	".html": true, ".chm": true, ".rtf": true, ".doc": true,
	".docx": true, ".ico": true,
}

// uninstallerPattern matches the names and targets installers give
// their removal tools.
var uninstallerPattern = regexp.MustCompile(
	`(?i)(^|[\\/ ])(unins\w*|uninstall\w*|remove \w+)`,
)

// edgeAppURL pulls the start page out of an Edge web app's arguments.
var edgeAppURL = regexp.MustCompile(`--app-url=(\S+)`)

// systemTool is one stock program of System32: its file and the name
// Windows shows for it, since the file name ("calc", "mspaint") is not
// what anyone types to find it.
type systemTool struct {
	file string
	name string
}

// systemTools are the stock programs of System32 worth listing by
// typing, the Windows counterpart of /System/Applications. explorer.exe
// is not one of them: it lives in the Windows folder, not in System32.
var systemTools = []systemTool{
	{file: "notepad.exe", name: "Notepad"},
	{file: "calc.exe", name: "Calculator"},
	{file: "mspaint.exe", name: "Paint"},
	{file: "cmd.exe", name: "Command Prompt"},
	{file: "control.exe", name: "Control Panel"},
	{file: "SnippingTool.exe", name: "Snipping Tool"},
	{file: "charmap.exe", name: "Character Map"},
}

// targetBase is the last element of a shortcut target. Targets are
// Windows paths even when this code runs elsewhere (the tests do), where
// filepath would not split on a backslash.
func targetBase(target string) string {
	cut := strings.LastIndexAny(target, `\/`)

	return target[cut+1:]
}

// isEdgeShortcut recognises a web app installed from Edge: a shortcut
// to msedge_proxy.exe (or msedge.exe) with an app id.
func isEdgeShortcut(link ShellLink) (string, bool) {
	base := strings.ToLower(targetBase(link.Target))
	if base != "msedge_proxy.exe" && base != "msedge.exe" {
		return "", false
	}

	if !strings.Contains(link.Args, "--app-id=") {
		return "", false
	}

	match := edgeAppURL.FindStringSubmatch(link.Args)
	if match == nil {
		return "", true
	}

	return match[1], true
}

// keepShortcut says whether a shortcut belongs on the grid: not an
// uninstaller, not a document, not a folder.
func keepShortcut(name string, link ShellLink) bool {
	if uninstallerPattern.MatchString(name) ||
		uninstallerPattern.MatchString(link.Target) {
		return false
	}

	ext := strings.ToLower(filepath.Ext(targetBase(link.Target)))
	if documentExtensions[ext] {
		return false
	}

	// A target without an extension is a folder (or nothing hopto can
	// name); an unreadable shortcut has no target and is kept.
	return link.Target == "" || ext != ""
}

// startMenuFolders is the root and its first level of subfolders:
// installers make one folder per product, rarely deeper.
func startMenuFolders(root string) []string {
	folders := []string{root}

	entries, err := os.ReadDir(root)
	if err != nil {
		return folders
	}

	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, filepath.Join(root, entry.Name()))
		}
	}

	return folders
}

// shortcutsIn lists the .lnk files directly inside folder, in the order
// the file system returns them. An unreadable folder has none.
func shortcutsIn(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}

	paths := []string{}
	for _, entry := range entries {
		isShortcut := !entry.IsDir() &&
			strings.EqualFold(filepath.Ext(entry.Name()), ".lnk")
		if isShortcut {
			paths = append(paths, filepath.Join(folder, entry.Name()))
		}
	}

	return paths
}

// InspectShortcut reads one .lnk into an App. A shortcut the parser
// refuses still comes back usable, with its file name and its path,
// and the error says why.
func InspectShortcut(path string) (App, error) {
	base := filepath.Base(path)
	app := App{
		Path:     path,
		Name:     strings.TrimSuffix(base, filepath.Ext(base)),
		IconPath: path,
		Source:   SourceApplications,
	}

	if !strings.EqualFold(filepath.Ext(path), ".lnk") {
		return app, nil
	}

	link, err := ReadShellLink(path)
	if err != nil {
		return app, err
	}

	if url, ok := isEdgeShortcut(link); ok {
		app.Source = SourceEdge
		app.Description = "Edge"

		if host, err := library.HostOf(url); err == nil {
			app.Host = host
			app.Description = host
			app.URL = url
		}
	}

	return app, nil
}

// ScanStartMenu lists the programs of the Start Menu roots, plus the
// stock tools of systemRoot when it is set. A target seen twice is
// listed once, from the first shortcut found.
func ScanStartMenu(roots []string, systemRoot string) []App {
	apps := []App{}
	seenTarget := map[string]bool{}

	for _, root := range roots {
		for _, folder := range startMenuFolders(root) {
			for _, path := range shortcutsIn(folder) {
				// An unreadable shortcut has an empty link: it is kept
				// under its file name rather than hidden.
				link, _ := ReadShellLink(path)
				if !keepShortcut(filepath.Base(path), link) {
					continue
				}

				// Edge apps are listed by the Edge scan, like on macOS.
				// They are decided first so that their shared msedge.exe
				// target does not hide the plain Edge shortcut.
				app, _ := InspectShortcut(path)
				if app.Source == SourceEdge {
					continue
				}

				target := strings.ToLower(link.Target)
				if target != "" && seenTarget[target] {
					continue
				}

				seenTarget[target] = true

				apps = append(apps, app)
			}
		}
	}

	apps = append(apps, systemToolsIn(systemRoot)...)

	sortByName(apps)
	assignIDs(apps, library.PrefixApplications)

	return apps
}

// systemToolsIn lists the stock tools that exist under systemRoot, under
// their display names (InspectShortcut names a plain exe after its
// file); an empty root, as on a test machine, lists none.
func systemToolsIn(systemRoot string) []App {
	tools := []App{}
	if systemRoot == "" {
		return tools
	}

	for _, tool := range systemTools {
		path := filepath.Join(systemRoot, tool.file)
		if _, err := os.Stat(path); err == nil {
			app, _ := InspectShortcut(path)
			app.Name = tool.name
			tools = append(tools, app)
		}
	}

	return tools
}

// ScanEdgeShortcuts lists the Edge web apps of a Start Menu root.
func ScanEdgeShortcuts(dir string) []App {
	apps := []App{}

	for _, folder := range startMenuFolders(dir) {
		for _, path := range shortcutsIn(folder) {
			app, _ := InspectShortcut(path)
			if app.Source == SourceEdge {
				apps = append(apps, app)
			}
		}
	}

	sortByName(apps)
	assignIDs(apps, library.PrefixEdge)

	return apps
}
