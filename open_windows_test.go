package main

import "testing"

// openArguments turns the `open` dialect the engine speaks into a
// ShellExecute call: -b browser url, -R path, -t path, or the thing.
func TestOpenArguments(t *testing.T) {
	cases := []struct {
		args               []string
		verb, file, params string
		fails              bool
	}{
		{
			args: []string{"https://example.org"},
			verb: "open", file: "https://example.org",
		},
		{
			args: []string{`C:\Users\someone\x.lnk`},
			verb: "open", file: `C:\Users\someone\x.lnk`,
		},
		{
			args: []string{
				"-b", `C:\Browsers\firefox.exe`, "https://example.org",
			},
			verb: "open", file: `C:\Browsers\firefox.exe`,
			params: "https://example.org",
		},
		{
			args: []string{"-R", `C:\Apps\x.exe`},
			verb: "open", file: "explorer.exe",
			params: `/select,"C:\Apps\x.exe"`,
		},
		{
			args: []string{"-t", `C:\Users\someone\library.toml`},
			verb: "open", file: "notepad.exe",
			params: `"C:\Users\someone\library.toml"`,
		},
		{args: []string{"-a", "Safari", "x"}, fails: true},
		{args: nil, fails: true},
	}

	for _, c := range cases {
		verb, file, params, err := openArguments(c.args)
		if c.fails {
			if err == nil {
				t.Errorf("%v: want an error", c.args)
			}

			continue
		}

		if err != nil || verb != c.verb || file != c.file ||
			params != c.params {
			t.Errorf("%v: got %q %q %q %v", c.args, verb, file, params, err)
		}
	}
}
