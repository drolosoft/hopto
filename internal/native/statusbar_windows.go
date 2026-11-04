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

// SetIcon does nothing: the tray shows the app icon, as every tray
// icon on Windows does, and the setting behind it is for macOS.
func (StatusBar) SetIcon(_ Icon) {}

// SetChecked moves the tick of one entry; the next click shows it.
func (StatusBar) SetChecked(tag int, on bool) {
	thread.mu.Lock()
	defer thread.mu.Unlock()

	setChecked(thread.items, tag, on)
}

// setChecked moves the tick of tag wherever it is, submenus included.
func setChecked(items []MenuItem, tag int, on bool) {
	for index := range items {
		if items[index].Tag == tag {
			items[index].Checked = on
		}

		setChecked(items[index].Children, tag, on)
	}
}
