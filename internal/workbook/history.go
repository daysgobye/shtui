package workbook

import "fmt"

const historyLimit = 100

type historyEntry struct {
	undo func() error
	redo func() error
}

type historyState struct {
	undo []historyEntry
	redo []historyEntry
}

// Undo restores the previous reversible operation in this session.
func Undo(session *Session) error {
	if session == nil {
		return fmt.Errorf("undo: session is nil")
	}
	if len(session.history.undo) == 0 {
		return fmt.Errorf("undo: no operation to undo")
	}
	last := len(session.history.undo) - 1
	entry := session.history.undo[last]
	if err := entry.undo(); err != nil {
		return fmt.Errorf("undo operation: %w", err)
	}
	session.history.undo = session.history.undo[:last]
	session.history.redo = appendBounded(session.history.redo, entry)
	return nil
}

// Redo reapplies the most recently undone operation in this session.
func Redo(session *Session) error {
	if session == nil {
		return fmt.Errorf("redo: session is nil")
	}
	if len(session.history.redo) == 0 {
		return fmt.Errorf("redo: no operation to redo")
	}
	last := len(session.history.redo) - 1
	entry := session.history.redo[last]
	if err := entry.redo(); err != nil {
		return fmt.Errorf("redo operation: %w", err)
	}
	session.history.redo = session.history.redo[:last]
	session.history.undo = appendBounded(session.history.undo, entry)
	return nil
}

func recordHistory(session *Session, entry historyEntry) {
	session.history.undo = appendBounded(session.history.undo, entry)
	session.history.redo = nil
}

func appendBounded(entries []historyEntry, entry historyEntry) []historyEntry {
	entries = append(entries, entry)
	if len(entries) > historyLimit {
		copy(entries, entries[len(entries)-historyLimit:])
		entries = entries[:historyLimit]
	}
	return entries
}
