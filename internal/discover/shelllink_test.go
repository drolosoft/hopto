package discover

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// linkFlags of the header (MS-SHLLINK 2.1.1). The test keeps its own copy
// so that a wrong constant in the parser cannot hide behind the writer.
const (
	testHasIDList   = 1 << 0
	testHasLinkInfo = 1 << 1
	testHasName     = 1 << 2
	testHasRelative = 1 << 3
	testHasWorkDir  = 1 << 4
	testHasArgs     = 1 << 5
	testHasIcon     = 1 << 6
	testIsUnicode   = 1 << 7
)

// utf16String is a StringData field: a character count and UTF-16.
func utf16String(text string) []byte {
	units := utf16.Encode([]rune(text))

	out := binary.LittleEndian.AppendUint16(nil, uint16(len(units)))
	for _, unit := range units {
		out = binary.LittleEndian.AppendUint16(out, unit)
	}

	return out
}

// linkBytes writes the smallest shortcut that holds the given pieces:
// a LinkInfo with an ANSI local path when target is set, then the
// StringData fields, then an optional environment block.
func linkBytes(
	target, args, icon string,
	iconIndex int32,
	env string,
) []byte {
	flags := uint32(testIsUnicode)
	if target != "" {
		flags |= testHasLinkInfo
	}

	if args != "" {
		flags |= testHasArgs
	}

	if icon != "" {
		flags |= testHasIcon
	}

	var out bytes.Buffer

	// ShellLinkHeader (2.1): size, class id, flags and the icon index.
	header := make([]byte, 76)
	binary.LittleEndian.PutUint32(header[0:], 76)
	copy(header[4:], []byte{
		0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
	})
	binary.LittleEndian.PutUint32(header[20:], flags)
	binary.LittleEndian.PutUint32(header[56:], uint32(iconIndex))
	out.Write(header)

	if target != "" {
		// LinkInfo (2.3) with the short 28-byte header: size, header
		// size, flags, then the offsets of the base path (byte 16) and
		// of the suffix (byte 24), both counted from the structure start.
		path := append([]byte(target), 0)
		suffix := []byte{0}
		size := uint32(28 + len(path) + len(suffix))
		info := make([]byte, 28)
		binary.LittleEndian.PutUint32(info[0:], size)
		binary.LittleEndian.PutUint32(info[4:], 28)
		binary.LittleEndian.PutUint32(info[8:], 1)
		binary.LittleEndian.PutUint32(info[16:], 28)
		binary.LittleEndian.PutUint32(info[24:], uint32(28+len(path)))
		out.Write(info)
		out.Write(path)
		out.Write(suffix)
	}

	if args != "" {
		out.Write(utf16String(args))
	}

	if icon != "" {
		out.Write(utf16String(icon))
	}

	if env != "" {
		// EnvironmentVariableDataBlock (2.5.4): 260 ANSI bytes, then
		// 260 UTF-16 characters, after the size and the signature.
		block := make([]byte, 8+260+520)
		binary.LittleEndian.PutUint32(block[0:], uint32(len(block)))
		binary.LittleEndian.PutUint32(block[4:], 0xA0000001)
		copy(block[8:], env)

		units := utf16.Encode([]rune(env))
		for index, unit := range units {
			binary.LittleEndian.PutUint16(block[268+2*index:], unit)
		}

		out.Write(block)
	}

	// TerminalBlock (2.5): a size below four ends the extra data.
	out.Write([]byte{0, 0, 0, 0})

	return out.Bytes()
}

// TestParseShellLinkReadsLinkInfoArgsAndIcon checks the three pieces
// hopto keeps from a shortcut that carries a LinkInfo.
func TestParseShellLinkReadsLinkInfoArgsAndIcon(t *testing.T) {
	data := linkBytes(
		`C:\Program Files\Foo\foo.exe`,
		"--flag",
		`C:\Program Files\Foo\foo.ico`,
		2,
		"",
	)

	link, err := ParseShellLink(data)
	if err != nil {
		t.Fatal(err)
	}

	if link.Target != `C:\Program Files\Foo\foo.exe` ||
		link.Args != "--flag" {
		t.Errorf("got %+v", link)
	}

	if link.IconLocation != `C:\Program Files\Foo\foo.ico` ||
		link.IconIndex != 2 {
		t.Errorf("icon: got %+v", link)
	}
}

// TestReadShellLinkExpandsTheEnvironmentBlock covers a system shortcut:
// no LinkInfo, the target only in the environment block.
func TestReadShellLinkExpandsTheEnvironmentBlock(t *testing.T) {
	t.Setenv("ProgramFiles", `C:\Program Files`)

	path := filepath.Join(t.TempDir(), "foo.lnk")
	data := linkBytes("", "", "", 0, `%ProgramFiles%\Foo\foo.exe`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	link, err := ReadShellLink(path)
	if err != nil {
		t.Fatal(err)
	}

	if link.Target != `C:\Program Files\Foo\foo.exe` {
		t.Errorf("target %q", link.Target)
	}
}

// TestReadShellLinkResolvesARelativePathAgainstTheShortcut checks that
// a target that is only a RELATIVE_PATH is taken from the .lnk folder.
// Only the base name is compared: filepath.Join uses the separator of
// the OS the test runs on.
func TestReadShellLinkResolvesARelativePathAgainstTheShortcut(
	t *testing.T,
) {
	folder := t.TempDir()
	path := filepath.Join(folder, "foo.lnk")

	// A header with the relative-path flag and its StringData, nothing
	// else: the parser has to fall back to it for the target.
	data := linkBytes("", "", "", 0, "")[:76]
	flags := uint32(testIsUnicode | testHasRelative)
	binary.LittleEndian.PutUint32(data[20:], flags)
	data = append(data, utf16String("bin")...)
	data = append(data, 0, 0, 0, 0)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	link, err := ReadShellLink(path)
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(link.Target) != "bin" {
		t.Errorf("target %q", link.Target)
	}

	if !strings.HasPrefix(link.Target, folder) {
		t.Errorf("target %q is not under %q", link.Target, folder)
	}
}

// TestParseShellLinkRejectsGarbage is Review Focus 1: garbage is
// refused, never a panic.
func TestParseShellLinkRejectsGarbage(t *testing.T) {
	inputs := [][]byte{
		nil,
		[]byte("hello"),
		linkBytes("x", "", "", 0, "")[:40],
		bytes.Repeat([]byte{0xFF}, 200),
	}

	for _, data := range inputs {
		if _, err := ParseShellLink(data); err == nil {
			t.Errorf("%d bytes: accepted", len(data))
		}
	}
}

// TestParseShellLinkSurvivesEveryTruncation cuts a full shortcut at
// every length: each cut must return, with an error or not, but never
// panic (run it with -race to hunt a bounds slip).
func TestParseShellLinkSurvivesEveryTruncation(t *testing.T) {
	full := linkBytes(
		`C:\Program Files\Foo\foo.exe`,
		"--flag",
		`C:\Program Files\Foo\foo.ico`,
		1,
		`%ProgramFiles%\Foo\foo.exe`,
	)

	for length := range full {
		// The result is dropped on purpose: only the absence of a
		// panic is under test here.
		_, _ = ParseShellLink(full[:length])
	}
}

// TestParseShellLinkRealFiles reads the real shortcuts of a Windows 11
// VM (task 1 of plan 5). They are skipped until that VM has produced
// them.
func TestParseShellLinkRealFiles(t *testing.T) {
	_, err := os.Stat("testdata/narrator.lnk")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("testdata/narrator.lnk is not there yet (task 1 VM)")
	}

	// System shortcuts keep %windir% unexpanded in their environment
	// block, so the test gives it a value that does not depend on the
	// machine running it.
	t.Setenv("windir", `C:\Windows`)

	link, err := ReadShellLink("testdata/narrator.lnk")
	if err != nil {
		t.Fatal(err)
	}

	// The target is a Windows path, so the base name is taken with
	// forward slashes to work on any OS running the test.
	base := filepath.Base(strings.ReplaceAll(link.Target, `\`, "/"))
	if !strings.EqualFold(base, "narrator.exe") {
		t.Errorf("narrator target %q", link.Target)
	}

	if _, err := os.Stat("testdata/edge-app.lnk"); err == nil {
		edge, err := ReadShellLink("testdata/edge-app.lnk")
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(edge.Args, "--app-id=") {
			t.Errorf("edge args %q", edge.Args)
		}
	}
}
