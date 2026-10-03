package app

import "sync"

// hotkeyStatuses keeps what RegisterEventHotKey answered for each id.
// It is global because the answer arrives through a C callback that
// knows nothing of the App; the welcome reads it from here.
var hotkeyStatuses = struct {
	sync.Mutex
	byID map[uint32]int32
}{byID: map[uint32]int32{}}

// recordHotkeyStatus stores the answer for one shortcut (0 is success).
func recordHotkeyStatus(id uint32, status int32) {
	hotkeyStatuses.Lock()
	defer hotkeyStatuses.Unlock()

	hotkeyStatuses.byID[id] = status
}

// hotkeyStatus reads the answer for one shortcut; known is false until
// Carbon has answered.
func hotkeyStatus(id uint32) (status int32, known bool) {
	hotkeyStatuses.Lock()
	defer hotkeyStatuses.Unlock()

	status, known = hotkeyStatuses.byID[id]

	return status, known
}
