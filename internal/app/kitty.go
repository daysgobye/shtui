package app

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	kittyAltScreenEnter = "\x1b[?1049h"
	kittyAltScreenExit  = "\x1b[?1049l"
	kittyEnableKeys     = "\x1b[>1u"
	kittyRestoreKeys    = "\x1b[<u"
)

type kittyTerminalWriter struct {
	*os.File
	protocol *kittyProtocolWriter
}

var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = (*kittyTerminalWriter)(nil)

func newKittyTerminalWriter(output *os.File) *kittyTerminalWriter {
	return &kittyTerminalWriter{
		File:     output,
		protocol: newKittyProtocolWriter(output),
	}
}

func (writer *kittyTerminalWriter) Write(data []byte) (int, error) {
	return writer.protocol.Write(data)
}

type kittyProtocolWriter struct {
	output  io.Writer
	mutex   sync.Mutex
	enabled bool
}

func newKittyProtocolWriter(output io.Writer) *kittyProtocolWriter {
	return &kittyProtocolWriter{output: output}
}

func (writer *kittyProtocolWriter) Write(data []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()

	remaining := data
	for len(remaining) > 0 {
		enter := bytes.Index(remaining, []byte(kittyAltScreenEnter))
		exit := bytes.Index(remaining, []byte(kittyAltScreenExit))
		if enter < 0 && exit < 0 {
			if err := writeTerminal(writer.output, remaining); err != nil {
				return 0, err
			}
			break
		}

		if enter >= 0 && (exit < 0 || enter < exit) {
			end := enter + len(kittyAltScreenEnter)
			if err := writeTerminal(writer.output, remaining[:end]); err != nil {
				return 0, err
			}
			remaining = remaining[end:]
			if !writer.enabled {
				if err := writeTerminal(writer.output, []byte(kittyEnableKeys)); err != nil {
					return 0, err
				}
				writer.enabled = true
			}
			continue
		}

		exitEnd := exit + len(kittyAltScreenExit)
		if err := writeTerminal(writer.output, remaining[:exit]); err != nil {
			return 0, err
		}
		if writer.enabled {
			if err := writeTerminal(writer.output, []byte(kittyRestoreKeys)); err != nil {
				return 0, err
			}
			writer.enabled = false
		}
		if err := writeTerminal(writer.output, remaining[exit:exitEnd]); err != nil {
			return 0, err
		}
		remaining = remaining[exitEnd:]
	}
	return len(data), nil
}

func writeTerminal(output io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := output.Write(data)
		data = data[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func withKittyKeyboardProtocol(output *os.File) tea.ProgramOption {
	return tea.WithOutput(newKittyTerminalWriter(output))
}

func filterKittyKey(_ tea.Model, message tea.Msg) tea.Msg {
	unknown, ok := message.(interface{ String() string })
	if !ok {
		return message
	}
	sequence, ok := parseUnknownCSI(unknown.String())
	if !ok {
		return message
	}
	keyCode, modifiers, ok := parseKittyKey(sequence)
	if !ok {
		return message
	}
	key, ok := mapKittyKey(keyCode, modifiers)
	if !ok {
		return message
	}
	return key
}

func mapKittyKey(keyCode, modifiers int) (tea.KeyMsg, bool) {
	switch keyCode {
	case 9:
		switch modifiers {
		case 0:
			return tea.KeyMsg{Type: tea.KeyTab}, true
		case 1:
			return tea.KeyMsg{Type: tea.KeyShiftTab}, true
		}
	case 13:
		if modifiers == 0 {
			return tea.KeyMsg{Type: tea.KeyEnter}, true
		}
	case 27:
		if modifiers == 0 {
			return tea.KeyMsg{Type: tea.KeyEsc}, true
		}
	case 57354:
		switch modifiers {
		case 0:
			return tea.KeyMsg{Type: tea.KeyPgUp}, true
		case 4:
			return tea.KeyMsg{Type: tea.KeyCtrlPgUp}, true
		}
	case 57355:
		switch modifiers {
		case 0:
			return tea.KeyMsg{Type: tea.KeyPgDown}, true
		case 4:
			return tea.KeyMsg{Type: tea.KeyCtrlPgDown}, true
		}
	}

	if keyCode < 'a' || keyCode > 'z' {
		return tea.KeyMsg{}, false
	}
	if modifiers == 4 {
		return tea.KeyMsg{Type: tea.KeyType(keyCode & 0x1f)}, true
	}
	if modifiers == 5 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+shift+" + string(rune(keyCode)))}, true
	}
	return tea.KeyMsg{}, false
}

func parseUnknownCSI(message string) ([]byte, bool) {
	const prefix, suffix = "?CSI[", "]?"
	if !strings.HasPrefix(message, prefix) || !strings.HasSuffix(message, suffix) {
		return nil, false
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(message, prefix), suffix)
	if encoded == "" {
		return nil, false
	}
	fields := strings.Fields(encoded)
	sequence := make([]byte, len(fields))
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 || value > 255 {
			return nil, false
		}
		sequence[index] = byte(value)
	}
	return sequence, true
}

func parseKittyKey(sequence []byte) (int, int, bool) {
	if len(sequence) < 2 || sequence[len(sequence)-1] != 'u' {
		return 0, 0, false
	}
	parameters := strings.Split(string(sequence[:len(sequence)-1]), ";")
	if len(parameters) != 2 {
		return 0, 0, false
	}
	keyCode, err := strconv.Atoi(parameters[0])
	if err != nil || keyCode < 0 {
		return 0, 0, false
	}
	encodedModifiers, err := strconv.Atoi(parameters[1])
	if err != nil || encodedModifiers < 1 {
		return 0, 0, false
	}
	return keyCode, encodedModifiers - 1, true
}
