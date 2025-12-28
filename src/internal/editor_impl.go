package internal

import (
	"io"
	"os"
	"strings"

	gc "github.com/gbin/goncurses"

	src "text_editor"
)

const (
	// rw-rw-rw-
	cReadWriteFileMode = 0666

	// Colors.
	cDebugColor = 1
)

func NewEditor(window *gc.Window, filePath string) (src.Editor, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, cReadWriteFileMode)
	if err != nil {
		return nil, err
	}
	e := &editorImpl{window: window, file: file}

	gc.InitPair(cDebugColor, gc.C_RED, gc.C_BLACK)
	// Initial update of window.
	e.sync()
	return e, nil
}

type editorImpl struct {
	window *gc.Window
	file   *os.File

	// Immutable state.
	maxHeight, maxWidth int

	// Mutable state.
	lineLengths      []int // How many chars on are on the ith line of the file.
	verbose          bool
	cursorX, cursorY int
}

var _ src.Editor = (*editorImpl)(nil)

func (e *editorImpl) Handle(key gc.Key) error {
	switch k := gc.KeyString(key); k {
	case "j":
		e.moveCursor(1 /*dy*/, 0 /*dx*/)
	case "k":
		e.moveCursor(-1 /*dy*/, 0 /*dx*/)
	case "l":
		e.moveCursor(0 /*dy*/, 1 /*dx*/)
	case "h":
		e.moveCursor(0 /*dy*/, -1 /*dx*/)
	case "0":
		e.cursorX = 0
	case "v":
		e.verbose = !e.verbose
	}
	e.sync()
	return nil
}

func (e *editorImpl) Close() {
	e.file.Close()
}

func (e *editorImpl) moveCursor(dy int, dx int) {
	cursorY, cursorX := e.window.CursorYX()
	maxY, maxX := e.window.MaxYX()
	newY, newX := cursorY+dy, cursorX+dx
	if newY < 0 || newY >= maxY || newX < 0 || newX >= maxX {
		// Don't go off-screen.
		return
	}
	if newY >= len(e.lineLengths) {
		// Don't go past the last line in the file.
		newY = len(e.lineLengths) - 1
	}
	if newX >= e.lineLengths[newY] {
		// Don't go past the last char in the current line.
		newX = e.lineLengths[newY] - 1
		if newX < 0 {
			newX = 0
		}
	}
	e.cursorY, e.cursorX = newY, newX
}

func (e *editorImpl) sync() {
	defer e.window.Refresh()
	defer func() {
		e.window.Move(e.cursorY, e.cursorX)
	}()
	// Will clear the STDOUT file and write whatever is viewable.
	e.clearDisplay()
	e.updateWindow()
	//e.updateWindowStub()
}

func (e *editorImpl) clearDisplay() {
	e.window.Erase()
}

func (e *editorImpl) updateWindowStub() {
	e.window.Print("dummy")
}

func (e *editorImpl) updateWindow() {
	contents := e.getFileContents()
	lineLengths := make([]int, len(contents))
	maxY, _ := e.window.MaxYX()
	for i := range maxY {
		if i < len(contents) {
			line := contents[i]
			lineLengths[i] = len(line)
			e.window.Println(line)
		} else {
			// Empty lines get special UI.
			e.window.AttrOn(gc.A_DIM)
			e.window.Println("~")
			e.window.AttrOff(gc.A_DIM)
		}
	}
	e.lineLengths = lineLengths
	e.window.Println()
	if e.verbose {
		// Print debug output.
		e.window.ColorOn(cDebugColor)
		e.window.Println("DEBUG: ")
		e.window.Printf("file has %d lines; ", len(contents))
		e.window.Printf("current line has %d chars; ", len(contents[e.cursorY]))
		e.window.Printf("cursor is at (x=%d,y=%d)", e.cursorX, e.cursorY)
		e.window.ColorOff(cDebugColor)
	}
}

// Each string is the entire row. The row does NOT contain the ending newline.
func (e *editorImpl) getFileContents() []string {
	// Make sure file is being read from beginning.
	e.file.Seek(0 /*offset*/, io.SeekStart)
	contents, err := io.ReadAll(e.file)
	if err != nil {
		panic(err)
	}

	fileContents := []string{}
	currRow := strings.Builder{}
	for _, b := range contents {
		if b == '\n' {
			// Line break, meaning we update a new row.
			fileContents = append(fileContents, currRow.String())
			currRow = strings.Builder{}
		} else {
			currRow.WriteByte(b)
		}
	}
	return fileContents
}
