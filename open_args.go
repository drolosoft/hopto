package main

import (
	"errors"
	"fmt"
)

// errOpenFlag answers a flag of `open` this adapter does not know.
var errOpenFlag = errors.New("unsupported open flag")

// openArguments maps the small dialect of `open` the engine speaks to a
// ShellExecute call: a bare URL or path opens as a double click would;
// -b runs the browser with the URL as its argument (secondary_browser
// is the path of an exe here); -R selects the file in the Explorer; -t
// opens the file in Notepad, which every Windows has and which never
// asks what to do with a .toml.
//
// It has no build tag because it is pure, so it can be tested anywhere.
func openArguments(args []string) (verb, file, params string, err error) {
	switch {
	case len(args) == 1 && args[0] != "":
		return "open", args[0], "", nil
	case len(args) == 3 && args[0] == "-b":
		return "open", args[1], args[2], nil
	case len(args) == 2 && args[0] == "-R":
		return "open", "explorer.exe", `/select,"` + args[1] + `"`, nil
	case len(args) == 2 && args[0] == "-t":
		return "open", "notepad.exe", `"` + args[1] + `"`, nil
	case len(args) > 0:
		return "", "", "", fmt.Errorf("%w: %v", errOpenFlag, args)
	default:
		return "", "", "", errors.New("open: nothing to open")
	}
}
