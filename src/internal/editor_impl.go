package internal

import (
	"io"
	"os"
	"strings"

	gc "github.com/gbin/goncurses"

	src "text_editor"
)

type EditorMode string

const (
	// rw-rw-rw-
	cReadWriteFileMode = 0666

	// Colors.
	COLOR_DEBUG = 1

	// Editor modes.
	NORMAL_MODE EditorMode = "NORMAL"
	INSERT_MODE EditorMode = "INSERT"

	// Escape sequences.
	ESC_KEY    = "\x1b"
	DELETE_KEY = "\x7f"
)

func NewEditor(window *gc.Window, filePath string, verbose bool) (src.Editor, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, cReadWriteFileMode)
	if err != nil {
		return nil, err
	}

	fileContents := getFileContents(file)
	e := &editorImpl{
		window:       window,
		file:         file,
		fileContents: fileContents,
		mode:         NORMAL_MODE,
		verbose:      verbose,
	}

	gc.InitPair(COLOR_DEBUG, gc.C_RED, gc.C_BLACK)
	// Initial update of window.
	e.sync()
	return e, nil
}

type editorImpl struct {
	window *gc.Window
	file   *os.File

	// Mutable state.
	mode             EditorMode
	fileContents     []string // Each element is a line from the source file without ending in '\n'.
	verbose          bool
	cursorX, cursorY int
}

var _ src.Editor = (*editorImpl)(nil)

func (e *editorImpl) Handle(key gc.Key) error {
	defer e.sync()
	switch e.mode {
	case NORMAL_MODE:
		return e.handleNormal(key)
	case INSERT_MODE:
		return e.handleInsert(key)
	default:
		return nil
	}
}

func (e *editorImpl) handleNormal(key gc.Key) error {
	switch k := gc.KeyString(key); k {
	case "q":
		e.Close()
		return io.EOF
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
	case "i":
		e.cursorX = e.normalizeCursorX(e.cursorX)
		e.mode = INSERT_MODE
	}
	return nil
}

func (e *editorImpl) handleInsert(key gc.Key) error {
	ch := gc.KeyString(key)
	switch ch {
	case ESC_KEY:
		// Swapping to NORMAL mode also decrements the x-pos by 1.
		e.mode = NORMAL_MODE
		e.cursorX = e.normalizeCursorX(e.cursorX - 1)
		return nil
	case DELETE_KEY:
		e.deleteChar()
		return nil
	default:
		e.insertChar(ch)
		return nil
	}
}

// Delete the character BEFORE the cursor position, and decrement the x-position by one.
// If the cursor is at the beginning of the line (x-pos = 0) and not on the first line (y-pos > 0),
// this is a special case and we:
// 1. Copy the entire contents of that line to the previous line.
// 2. Delete the current line (modify number of lines in file).
// 3. Decrement the cursor's y-pos by 1.
// 4. Update the cursor's x-pos to be whatever the end of the previous line was.
func (e *editorImpl) deleteChar() {
	currLine := e.fileContents[e.cursorY]
	if e.cursorX == 0 && e.cursorY == 0 {
		// Do nothing.
		return
	}
	if e.cursorX == 0 {
		// Special case. See docstring.
		prevLine := e.fileContents[e.cursorY-1]
		newLine := strings.Builder{}
		newLine.WriteString(prevLine)
		newLine.WriteString(currLine)
		// Replace the previous line.
		e.fileContents[e.cursorY-1] = newLine.String()
		// Remove the current line.
		e.fileContents = append(e.fileContents[:e.cursorY], e.fileContents[e.cursorY+1:]...)
		e.cursorY -= 1
		e.cursorX = len(prevLine)
		return
	}
	newLine := strings.Builder{}
	newLine.WriteString(currLine[:e.cursorX-1])
	newLine.WriteString(currLine[e.cursorX:])
	e.fileContents[e.cursorY] = newLine.String()
	e.cursorX -= 1
}

// Insert the character at the position of the cursor, and increment the x-position by one.
func (e *editorImpl) insertChar(ch string) {
	lineToInsertInto := e.fileContents[e.cursorY]
	newLine := strings.Builder{}
	newLine.WriteString(lineToInsertInto[:e.cursorX])
	newLine.WriteString(ch)
	newLine.WriteString(lineToInsertInto[e.cursorX:])
	e.fileContents[e.cursorY] = newLine.String()
	e.cursorX += 1
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
	if newY < 0 || newY >= maxY || newY >= len(e.fileContents) ||
		newX < 0 || newX >= maxX {
		// Don't go off-screen.
		// Don't go past the last line in the file.
		return
	}
	lineLength := len(e.fileContents[newY])
	if newX >= lineLength {
		// The newX is past the last char on the current line. That is valid (see the doc comment),
		// though we don't want to go any further than we are now.
		// So, if the x-pos is increasing, do not update it at all (set it to what it is currently).
		// Otherwise, if it's decreasing, set it to the second-to-last char on the current line. We
		// move it to the second-to-last instead of the last since the cursor is on the last char
		// from user's perspective.
		// Additionally, if the current line is empty, don't move it at all.
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
		e.window.Move(e.cursorY, e.normalizeCursorX(e.cursorX))
	}()
	e.updateWindow()
}

func (e *editorImpl) normalizeCursorX(x int) int {
	// In INSERT mode, it's expected for the cursor to be equal to the length of the current line.
	if e.mode == NORMAL_MODE && e.cursorX >= len(e.fileContents[e.cursorY]) {
		// Special handling of x-position. See moveCursorInternal for details.
		x = len(e.fileContents[e.cursorY]) - 1
	}
	if x < 0 {
		x = 0
	}
	return x
}

func (e *editorImpl) updateWindow() {
	// Update the window atomically by replacing it. This is more efficient than multiple Print calls
	// on the user-visible window, which may result in flashes.
	windowY, windowX := e.window.YX()
	maxY, maxX := e.window.MaxYX()
	newWindow, _ := gc.NewWindow(maxY, maxX, windowY, windowX)

	for i := range maxY {
		if i < len(e.fileContents) {
			line := e.fileContents[i]
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
	e.window.Println()
	if e.verbose {
		// Print debug output.
		newWindow.ColorOn(COLOR_DEBUG)
		newWindow.Println("DEBUG: ")
		newWindow.Printf("file length=%d lines; ", len(e.fileContents))
		newWindow.Printf("current line length=%d chars; ", len(e.fileContents[e.cursorY]))
		newWindow.Printf("cursor at (x=%d,y=%d); ", e.cursorX, e.cursorY)
		newWindow.Printf("mode=%s", e.mode)
		newWindow.ColorOff(COLOR_DEBUG)
	}
	e.window.Overwrite(newWindow)
}

// Each string is the entire row. The row does NOT contain the ending newline.
func getFileContents(file *os.File) []string {
	// Make sure file is being read from beginning.
	file.Seek(0 /*offset*/, io.SeekStart)
	contents, err := io.ReadAll(file)
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
