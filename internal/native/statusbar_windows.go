//go:build windows

package native

// StatusBar is the tray icon's menu. The icon itself is created by the
// native thread at start; this only keeps the list the menu is built
// from at each click.
type StatusBar struct{}

// Install keeps the items and makes sure the native thread is up.
func (StatusBar) Install(items []MenuItem) {
	startNative()

	native.mu.Lock()
	defer native.mu.Unlock()

	native.items = append([]MenuItem{}, items...)
}

// SetChecked moves the tick of one entry; the next click shows it.
func (StatusBar) SetChecked(tag int, on bool) {
	native.mu.Lock()
	defer native.mu.Unlock()

	for index := range native.items {
		if native.items[index].Tag == tag {
			native.items[index].Checked = on
		}
	}
}
