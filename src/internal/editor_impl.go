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

func NewEditor(window *gc.Window, filePath string, verbose bool) (src.Editor, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, cReadWriteFileMode)
	if err != nil {
		return nil, err
	}
	e := &editorImpl{window: window, file: file, verbose: verbose}

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
		e.moveCursorIncremental(1 /*dy*/, 0 /*dx*/)
	case "k":
		e.moveCursorIncremental(-1 /*dy*/, 0 /*dx*/)
	case "l":
		e.moveCursorIncremental(0 /*dy*/, 1 /*dx*/)
	case "h":
		e.moveCursorIncremental(0 /*dy*/, -1 /*dx*/)
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

func (e *editorImpl) moveCursorIncremental(dy int, dx int) {
	e.moveCursor(e.cursorY+dy, e.cursorX+dx)
}

// moveCursor handles the validation of the new cursor location, and applies safeguards if the cursor
// is attempted to be moved to an invalid position.
//
// The cursor's x-position that is stored here is not the actual position the cursor occupies. Instead,
// it's treated as the max possible position it may occupy, limited by the current line's length.
// For example, say the current line has 40 chars, and the cursor's x-pos is 30. If the cursor moves
// to a line with fewer chars, say 10, the stored x-pos is still 30, even though the cursor would
// actually occupy an x-pos of 9 (the max possible on a line of length 10). This is to preserve the
// x-pos on shorter lines so that when we return to larger lines, the x-pos "pops" back to 30.
func (e *editorImpl) moveCursor(newY int, newX int) {
	maxY, maxX := e.window.MaxYX()
	if newY < 0 || newY >= maxY || newY >= len(e.lineLengths) ||
		newX < 0 || newX >= maxX {
		// Don't go off-screen.
		// Don't go past the last line in the file.
		return
	}
	if newX >= e.lineLengths[newY] {
		// The newX is past the last char on the current line. That is valid (see the doc comment),
		// though we don't want to go any further than we are now.
		// So, if the x-pos is increasing, do not update it at all (set it to what it is currently).
		// Otherwise, if it's decreasing, set it to the second-to-last char on the current line. We
		// move it to the second-to-last instead of the last since the cursor is on the last char
		// from user's perspective.
		// Additionally, if the current line is empty, don't move it at all.
		lineLength := e.lineLengths[newY]
		if newX >= e.cursorX || lineLength == 0 {
			newX = e.cursorX
		} else {
			newX = lineLength - 2
		}
	}
	e.cursorY, e.cursorX = newY, newX
}

func (e *editorImpl) sync() {
	defer e.window.Refresh()
	defer func() {
		newX := e.cursorX
		if e.cursorX >= e.lineLengths[e.cursorY] {
			// Special handling of x-position. See moveCursorInternal for details.
			newX = e.lineLengths[e.cursorY] - 1
		}
		if newX < 0 {
			newX = 0
		}
		e.window.Move(e.cursorY, newX)
	}()
	e.updateWindow()
}

func (e *editorImpl) updateWindow() {
	// Update the window atomically by replacing it. This is more efficient than multiple Print calls
	// on the user-visible window, which may result in flashes.
	windowY, windowX := e.window.YX()
	maxY, maxX := e.window.MaxYX()
	newWindow, _ := gc.NewWindow(maxY, maxX, windowY, windowX)

	contents := e.getFileContents()
	lineLengths := make([]int, len(contents))
	for i := range maxY {
		if i < len(contents) {
			line := contents[i]
			lineLengths[i] = len(line)
			newWindow.Println(line)
		} else if i != maxY-1 {
			// There are no more file contents, so use a special UI to denote that these liens are
			// not present in the file. We do not do that on the last line, as the last line is used
			// for debug output.
			newWindow.AttrOn(gc.A_DIM)
			newWindow.Println("~")
			newWindow.AttrOff(gc.A_DIM)
		}
	}
	e.lineLengths = lineLengths
	e.window.Println()
	if e.verbose {
		// Print debug output.
		newWindow.ColorOn(cDebugColor)
		newWindow.Println("DEBUG: ")
		newWindow.Printf("file has %d lines; ", len(contents))
		newWindow.Printf("current line has %d chars; ", len(contents[e.cursorY]))
		newWindow.Printf("cursor is at (x=%d,y=%d)", e.cursorX, e.cursorY)
		newWindow.ColorOff(cDebugColor)
	}
	e.window.Overwrite(newWindow)
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
