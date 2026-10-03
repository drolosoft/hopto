package app

import "sync"

// hotkeyStatusOK is what the system answers for a shortcut that is now
// hopto's, on both systems; any other status is a refusal.
const hotkeyStatusOK int32 = 0

// hotkeyStatuses keeps what the system answered when each shortcut was
// registered (RegisterEventHotKey on macOS, RegisterHotKey on Windows).
// The native side reports it through Hooks.HotkeyRegistered, a plain
// function that knows nothing of the App, so the table lives here, and
// the welcome reads it from here.
var hotkeyStatuses = struct {
	sync.Mutex
	byID map[uint32]int32
}{byID: map[uint32]int32{}}

// recordHotkeyStatus stores the answer for one shortcut; startup hands
// it to native.Start as Hooks.HotkeyRegistered.
func recordHotkeyStatus(id uint32, status int32) {
	hotkeyStatuses.Lock()
	defer hotkeyStatuses.Unlock()

	hotkeyStatuses.byID[id] = status
}

// hotkeyStatus reads the answer for one shortcut; known is false until
// the system has answered.
func hotkeyStatus(id uint32) (status int32, known bool) {
	hotkeyStatuses.Lock()
	defer hotkeyStatuses.Unlock()

	status, known = hotkeyStatuses.byID[id]

	return status, known
}
