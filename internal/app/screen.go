package app

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/drolosoft/hopto/internal/atomicfile"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/platform"
)

// windowPerm keeps window.json private, like every other file in the
// data folder.
const windowPerm = 0o600

// windowState is window.json: the native package's id of the screen the
// panel was on when it last hid (the CGDirectDisplayID on macOS, a hash
// of the monitor's device name on Windows). The id survives a restart
// and a replug of the same monitor, which is what lets a new run put the
// panel back.
type windowState struct {
	Display uint32 `json:"display"`
}

// readWindowState is the remembered display, or 0 when there is none: a
// missing file is a first run, and a broken one is logged and forgotten
// (the next hide writes a good one), never a reason not to show.
func readWindowState(path string) uint32 {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}

	if err != nil {
		log.Printf("window state: %v", err)
		return 0
	}

	var state windowState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("window state %s: %v", path, err)
		return 0
	}

	return state.Display
}

// writeWindowState keeps display in window.json through atomicfile, so
// a crash half way never leaves a truncated file.
func writeWindowState(path string, display uint32) error {
	data, err := json.Marshal(windowState{Display: display})
	if err != nil {
		return err
	}

	return atomicfile.Write(path, data, windowPerm)
}

// screenChoice decides where the panel shows. "mouse" and "main" are
// taken as they are; "last" (and anything else, which checkSettings
// never lets through) is the remembered display while it is attached,
// and the main screen when nothing is remembered or it was unplugged:
// the panel must never be sent to a screen that is not there.
func screenChoice(
	setting string, remembered uint32, attached []uint32,
) (string, uint32) {
	if setting == library.ScreenMouse || setting == library.ScreenMain {
		return setting, 0
	}

	if remembered != 0 && slices.Contains(attached, remembered) {
		return library.ScreenLast, remembered
	}

	return library.ScreenMain, 0
}

// placeWindow centres the hidden window where the setting says, just
// before it is shown. Called with a.mu held, from a goroutine.
func (a *App) placeWindow() {
	setting := a.library.Snapshot().Settings.Screen
	attached := a.attachedDisplays()
	mode, display := screenChoice(setting, a.display, attached)

	log.Printf("window: placed by %s on display %d", mode, display)
	a.window.Center(mode, display)
}

// rememberDisplay notes the display the window is on just before it
// hides, and writes window.json when it changed, so the next show and
// the next run put the panel back there. A window on no screen (0)
// changes nothing. Called with a.mu held, from a goroutine.
func (a *App) rememberDisplay() {
	display := a.windowDisplay()
	if display == 0 || display == a.display {
		return
	}

	a.display = display
	log.Printf("window: display %d remembered", display)

	path := filepath.Join(a.dataDir, platform.WindowFile)
	if err := writeWindowState(path, display); err != nil {
		log.Printf("window state: %v", err)
	}
}
