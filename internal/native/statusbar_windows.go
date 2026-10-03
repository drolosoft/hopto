//go:build windows

package native

// StatusBar is the tray icon's menu. The icon itself is created by the
// native thread at start; this only keeps the list the menu is built
// from at each click.
type StatusBar struct{}

// Install keeps the items and makes sure the native thread is up.
func (StatusBar) Install(items []MenuItem) {
	startNative()

	thread.mu.Lock()
	defer thread.mu.Unlock()

	thread.items = append([]MenuItem{}, items...)
}

// SetChecked moves the tick of one entry; the next click shows it.
func (StatusBar) SetChecked(tag int, on bool) {
	thread.mu.Lock()
	defer thread.mu.Unlock()

	for index := range thread.items {
		if thread.items[index].Tag == tag {
			thread.items[index].Checked = on
		}
	}
}
