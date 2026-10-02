//go:build windows

package main

// statusBar is the tray icon's menu. The icon itself is created by the
// native thread at start; this only keeps the list the menu is built
// from at each click.
type statusBar struct{}

// Install keeps the items and makes sure the native thread is up.
func (statusBar) Install(items []menuItem) {
	startNative()

	native.mu.Lock()
	defer native.mu.Unlock()

	native.items = append([]menuItem{}, items...)
}

// SetChecked moves the tick of one entry; the next click shows it.
func (statusBar) SetChecked(tag int, on bool) {
	native.mu.Lock()
	defer native.mu.Unlock()

	for index := range native.items {
		if native.items[index].Tag == tag {
			native.items[index].Checked = on
		}
	}
}
