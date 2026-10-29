package app

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKittyProtocolOutputWriterBracketsAlternateScreen(t *testing.T) {
	var output bytes.Buffer
	writer := newKittyProtocolWriter(&output)
	_, _ = writer.Write([]byte("before"))
	_, _ = writer.Write([]byte("\x1b[?1049h"))
	_, _ = writer.Write([]byte("inside"))
	_, _ = writer.Write([]byte("\x1b[?1049l"))
	_, _ = writer.Write([]byte("after"))

	want := "before\x1b[?1049h\x1b[>1uinside\x1b[<u\x1b[?1049lafter"
	if got := output.String(); got != want {
		t.Fatalf("terminal output = %q, want %q", got, want)
	}
}

func TestKittyProtocolOutputPreservesTerminalFile(t *testing.T) {
	output, err := os.CreateTemp(t.TempDir(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	writer := any(newKittyTerminalWriter(output))
	terminal, ok := writer.(interface {
		io.ReadWriteCloser
		Fd() uintptr
	})
	if !ok {
		t.Fatal("output writer no longer exposes the underlying terminal file")
	}
	if got, want := terminal.Fd(), output.Fd(); got != want {
		t.Fatalf("output file descriptor = %d, want %d", got, want)
	}
}

func TestKittyKeyFilterDecodesControlShortcuts(t *testing.T) {
	for _, test := range []struct {
		name string
		csi  string
		want string
	}{
		{name: "copy", csi: "\x1b[99;6u", want: "ctrl+shift+c"},
		{name: "quit", csi: "\x1b[99;5u", want: "ctrl+c"},
		{name: "palette", csi: "\x1b[112;5u", want: "ctrl+p"},
		{name: "cut", csi: "\x1b[120;5u", want: "ctrl+x"},
		{name: "paste", csi: "\x1b[118;5u", want: "ctrl+v"},
		{name: "save", csi: "\x1b[115;5u", want: "ctrl+s"},
		{name: "undo", csi: "\x1b[122;5u", want: "ctrl+z"},
		{name: "redo", csi: "\x1b[121;5u", want: "ctrl+y"},
		{name: "find", csi: "\x1b[102;5u", want: "ctrl+f"},
		{name: "toolbar", csi: "\x1b[116;5u", want: "ctrl+t"},
		{name: "escape", csi: "\x1b[27;1u", want: "esc"},
		{name: "shift tab", csi: "\x1b[9;2u", want: "shift+tab"},
		{name: "ctrl page up", csi: "\x1b[57354;5u", want: "ctrl+pgup"},
		{name: "ctrl page down", csi: "\x1b[57355;5u", want: "ctrl+pgdown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := filterKittyKey(nil, unknownKittyCSI(test.csi))
			key, ok := message.(tea.KeyMsg)
			if !ok || key.String() != test.want {
				t.Fatalf("filtered key = %#v, want %q", message, test.want)
			}
		})
	}
}

func TestKittyCtrlCStillOpensQuitDialog(t *testing.T) {
	controller := NewController("")
	message := filterKittyKey(nil, unknownKittyCSI("\x1b[99;5u"))
	controller.Update(message)
	if !controller.quitDialog.open {
		t.Fatal("enhanced Ctrl+C did not open the quit dialog")
	}
}

func TestKittyKeyFilterLeavesOrdinaryMessagesUntouched(t *testing.T) {
	original := tea.KeyMsg{Type: tea.KeyCtrlC}
	if got := filterKittyKey(nil, original); !reflect.DeepEqual(got, original) {
		t.Fatalf("ordinary Ctrl+C message changed to %#v", got)
	}
}

func TestKittyKeyFilterPassesUnknownCSI(t *testing.T) {
	original := unknownKittyCSI("\x1b[99;7u")
	if got := filterKittyKey(nil, original); got != original {
		t.Fatalf("unsupported Kitty key changed to %#v", got)
	}
}

// unknownKittyCSI models Bubble Tea's internal unknown-CSI message, including its String form.
type unknownKittyCSI string

func (message unknownKittyCSI) String() string {
	sequence := []byte(message)
	sequence = bytes.TrimPrefix(sequence, []byte("\x1b["))
	values := make([]byte, len(sequence))
	for index, value := range sequence {
		values[index] = value
	}
	return fmtCSI(values)
}

func fmtCSI(values []byte) string {
	var result bytes.Buffer
	result.WriteString("?CSI[")
	for index, value := range values {
		if index > 0 {
			result.WriteByte(' ')
		}
		result.WriteString(strconv.Itoa(int(value)))
	}
	result.WriteString("]?")
	return result.String()
}
