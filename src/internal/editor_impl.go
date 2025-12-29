package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	gc "github.com/gbin/goncurses"

	"github.com/omarnabikhan/gim/src"
	"github.com/omarnabikhan/gim/src/internal/build_version"
)

type EditorMode string

const (
	// rw-rw-rw-
	cReadWriteFileMode = 0666

	// Colors.
	COLOR_DEFAULT = 100
	COLOR_DEBUG   = 101
	COLOR_BG      = 102

	// Color pairs.
	COLOR_PAIR_DEBUG   = 1
	COLOR_PAIR_DEFAULT = 2

	// Editor modes.
	NORMAL_MODE  EditorMode = "NORMAL"
	INSERT_MODE  EditorMode = "INSERT"
	COMMAND_MODE EditorMode = "COMMAND"

	// Escape sequences.
	ESC_KEY    = "\x1b"
	DELETE_KEY = "\x7f"
)

func NewEditor(window *gc.Window, filePath string, verbose bool) (src.Editor, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, cReadWriteFileMode)
	if err != nil {
		return nil, err
	}

	fileContents, lengthBytes := getFileContentsAndLen(file)
	e := &editorImpl{
		window:       window,
		file:         file,
		fileContents: fileContents,
		mode:         NORMAL_MODE,
		verbose:      verbose,
		userMsg:      fmt.Sprintf(`file "%s" %dL %dB`, file.Name(), len(fileContents), lengthBytes),
	}

	gc.InitColor(COLOR_DEFAULT, 900, 900, 900)
	gc.InitColor(COLOR_DEBUG, 887, 113, 63)
	gc.InitColor(COLOR_BG, 170, 170, 170)

	gc.InitPair(COLOR_PAIR_DEBUG, COLOR_DEBUG, COLOR_BG)
	gc.InitPair(COLOR_PAIR_DEFAULT, COLOR_DEFAULT, COLOR_BG)

	// Initial update of window.
	e.sync()
	return e, nil
}

type editorImpl struct {
	window *gc.Window
	file   *os.File

	// Mutable state.
	fileContents     []string // Each element is a line from the source file without ending in '\n'.
	fileLineOffset   int      // Which line of the file is being shown at the top of the screen.
	userMsg          string   // Shown to user at bottom of screen.
	mode             EditorMode
	verbose          bool
	cursorX, cursorY int
	commandBuffer    strings.Builder
}

var _ src.Editor = (*editorImpl)(nil)

func (e *editorImpl) Handle(key gc.Key) error {
	defer e.sync()
	switch e.mode {
	case NORMAL_MODE:
		return e.handleNormal(key)
	case INSERT_MODE:
		return e.handleInsert(key)
	case COMMAND_MODE:
		return e.handleCommand(key)
	default:
		return nil
	}
}

func (e *editorImpl) handleNormal(key gc.Key) error {
	switch k := gc.KeyString(key); k {
	case "j":
		// Move the cursor down.
		e.moveCursorVertical(1)
		return nil
	case "k":
		// Move the cursor up.
		e.moveCursorVertical(-1)
		return nil
	case "l":
		// Move the cursor right.
		e.moveCursorHorizontal(1)
		return nil
	case "h":
		// Move the cursor left.
		e.moveCursorHorizontal(-1)
		return nil
	case "0":
		// Move the cursor to the beginning of the current line.
		e.cursorX = 0
		return nil
	case "H":
		// Move the cursor to the highest position without scrolling.
		e.cursorY = 0
		return nil
	case "L":
		// Move the cursor to the lowest valid position without scrolling.
		newY := e.getMaxYForContent()
		if newY+e.fileLineOffset >= len(e.fileContents) {
			// Special case: we ran out of file. Instead, move the cursor to the last line of the file.
			newY = len(e.fileContents) - e.fileLineOffset - 1
		}
		e.cursorY = newY
		return nil
	case "v":
		// Toggle verbose mode.
		e.verbose = !e.verbose
		return nil
	case "o":
		// Insert an empty line after the current line, and swap to INSERT mode.
		currLineInd := e.getCurrLineInd()
		e.fileContents = append(
			e.fileContents[:currLineInd+1],
			append(
				[]string{""},
				e.fileContents[currLineInd+1:]...,
			)...,
		)
		e.moveCursorVertical(1)
		e.swapToInsertMode()
		return nil
	case "O":
		// Insert an empty line before the current line, and swap to INSERT mode.
		currLineInd := e.getCurrLineInd()
		e.fileContents = append(
			e.fileContents[:currLineInd],
			append(
				[]string{""},
				e.fileContents[currLineInd:]...,
			)...,
		)
		e.swapToInsertMode()
		return nil
	case "i":
		// Swap to INSERT mode.
		e.swapToInsertMode()
		return nil
	case ":":
		// Swap to CMD mode.
		e.mode = COMMAND_MODE
		e.userMsg = ":"
		return nil
	default:
		// Do nothing.
		return nil
	}
}

func (e *editorImpl) swapToInsertMode() {
	e.cursorX = e.normalizeCursorX(e.cursorX)
	e.mode = INSERT_MODE
	e.userMsg = "-- INSERT --"
}

func (e *editorImpl) moveCursorVertical(dy int) {
	newY := e.cursorY + dy
	numLinesInFile := len(e.fileContents)

	// Validation to prevent scrolling past file contents.
	if newY+e.fileLineOffset < 0 {
		// Nothing to scroll up to.
		return
	} else if newY+e.fileLineOffset >= numLinesInFile {
		// Nothing to scroll down to.
		return
	}

	// Handle valid scrolling. Scrolling also wipes the userMsg.
	if newY < 0 {
		// Scroll up one line.
		e.fileLineOffset -= 1
		newY = 0
		e.userMsg = ""
	} else if newY > e.getMaxYForContent() {
		e.fileLineOffset += 1
		// Keep the cursor on the same line (the last line).
		newY = e.getMaxYForContent()
		e.userMsg = ""
	}
	// Else, we're within the displayed content and can simply update the y-pos.
	e.cursorY = newY
}

// The cursor's x-position that is stored here is not the actual position the cursor occupies. Instead,
// it's treated as the max possible position it may occupy, limited by the current line's length.
// For example, say the current line has 40 chars, and the cursor's x-pos is 30. If the cursor moves
// to a line with fewer chars, say 10, the stored x-pos is still 30, even though the cursor would
// actually occupy an x-pos of 9 (the max possible on a line of length 10). This is to preserve the
// x-pos on shorter lines so that when we return to larger lines, the x-pos "pops" back to 30.
func (e *editorImpl) moveCursorHorizontal(dx int) {
	newX := e.cursorX + dx
	if newX < 0 {
		return
	}
	lineLength := len(e.fileContents[e.getCurrLineInd()])
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
	e.cursorX = newX
}

// Write the contents of the in-memory file to disc.
// TODO(omar): Very simple implementation of clear the file, then overwrite full contents. We can do
// better if we know that only some small portion of the file needs to change.
func (e *editorImpl) writeToDisc() error {
	defer e.file.Sync()

	// Clear the contents of the file.
	if err := os.Truncate(e.file.Name(), 0); err != nil {
		return err
	}

	// Write the new contents of file to disc.
	e.file.Seek(0 /*offset*/, io.SeekStart)
	// We collect in a []byte and do a single write for efficiency.
	contents := bytes.Buffer{}
	for _, line := range e.fileContents {
		contents.WriteString(line)
		contents.WriteString("\n")
	}
	n, err := e.file.Write(contents.Bytes())
	if err != nil {
		return err
	}
	// Update the display to say we wrote to disc.
	e.userMsg = fmt.Sprintf("%d bytes written to disc", n)
	return nil
}

func (e *editorImpl) handleInsert(key gc.Key) error {
	ch := gc.KeyString(key)
	switch ch {
	case ESC_KEY:
		// Swap to NORMAL model
		// Swapping decrements the x-pos by 1.
		e.mode = NORMAL_MODE
		e.cursorX = e.normalizeCursorX(e.cursorX - 1)
		e.userMsg = ""
		return nil
	case DELETE_KEY:
		// Delete the char before the cursor.
		e.deleteChar()
		return nil
	default:
		// Insert a char at the cursor.
		e.insertChar(ch)
		return nil
	}
}

// Handle the user inputting the delete key.
func (e *editorImpl) deleteChar() {
	currLineInd := e.getCurrLineInd()
	currLine := e.fileContents[currLineInd]
	if e.cursorX == 0 && currLineInd == 0 {
		// Do nothing.
		return
	}
	if e.cursorX == 0 {
		// If the cursor is at the beginning of the line (x-pos = 0) and not on the first line
		// (y-pos > 0), this is a special case and we:
		// 1. Copy the entire contents of that line to the previous line.
		// 2. Delete the current line (modify number of lines in file).
		// 3. Decrement the cursor's y-pos by 1.
		// 4. Update the cursor's x-pos to be whatever the end of the previous line was.
		prevLine := e.fileContents[currLineInd-1]
		newLine := strings.Builder{}
		newLine.WriteString(prevLine)
		newLine.WriteString(currLine)
		// Replace the previous line.
		e.fileContents[currLineInd-1] = newLine.String()
		// Remove the current line.
		e.fileContents = append(e.fileContents[:currLineInd], e.fileContents[currLineInd+1:]...)
		e.moveCursorVertical(-1)
		e.cursorX = len(prevLine)
		return
	}
	newLine := strings.Builder{}
	newLine.WriteString(currLine[:e.cursorX-1])
	newLine.WriteString(currLine[e.cursorX:])
	e.fileContents[currLineInd] = newLine.String()
	e.moveCursorHorizontal(-1)
}

// Handle the user inputting the ch key.
func (e *editorImpl) insertChar(ch string) {
	currLineInd := e.getCurrLineInd()
	currLine := e.fileContents[currLineInd]
	if ch == "enter" {
		// Upon pressing the "enter" key, the current line is split before and after the x-pos of
		// the cursor, and:
		// 1. The "before" part stays on the current line.
		// 2. The "after" part (includes cursor's x-pos) is pushed to a new.
		// 3. The cursor's x-pos becomes 0.
		// 4. The cursor's y-pos is incremented by 1.
		before, after := currLine[:e.cursorX], currLine[e.cursorX:]
		e.fileContents[currLineInd] = before
		e.fileContents = append(
			e.fileContents[:currLineInd+1],
			append([]string{after}, e.fileContents[currLineInd+1:]...)...,
		)
		e.cursorX = 0
		e.moveCursorVertical(1)
		return
	}
	newLine := strings.Builder{}
	newLine.WriteString(currLine[:e.cursorX])
	newLine.WriteString(ch)
	newLine.WriteString(currLine[e.cursorX:])
	e.fileContents[currLineInd] = newLine.String()
	e.moveCursorHorizontal(1)
}

func (e *editorImpl) handleCommand(key gc.Key) error {
	ch := gc.KeyString(key)
	switch ch {
	case ESC_KEY:
		// Cancel the command.
		e.userMsg = ""
		e.mode = NORMAL_MODE
		return nil
	case DELETE_KEY:
		// Delete the last char in the command. If the command is empty, then swap to NORMAL mode too.
		if len(e.userMsg) == 1 {
			e.userMsg = ""
			e.mode = NORMAL_MODE
			return nil
		}
		e.userMsg = e.userMsg[:len(e.userMsg)-1]
		return nil
	case "enter":
		// Trim the beginning ":"
		command := e.userMsg[1:]
		e.userMsg = ""
		defer func() { e.mode = NORMAL_MODE }()
		return e.handleCommandEntered(command)
	default:
		// Just add to command buffer.
		e.userMsg += ch
		return nil
	}
}

func (e *editorImpl) handleCommandEntered(command string) error {
	switch command {
	case "w":
		// Write the contents of the in-memory buffer to disc, and s
		return e.writeToDisc()
	case "q":
		// Quit the program.
		e.Close()
		return io.EOF
	default:
		e.userMsg = fmt.Sprintf("unrecognized command: %s", command)
		return nil
	}
}

func (e *editorImpl) Close() {
	e.file.Close()
}

func (e *editorImpl) sync() {
	defer e.window.Refresh()
	defer func() {
		if e.mode == COMMAND_MODE {
			// Overwrite whatever the cursorY and cursorX are to show the cursor at the bottom of
			// the screen.
			e.window.Move(e.getMaxYForContent()+2, len(e.userMsg))
		} else {
			e.window.Move(e.cursorY, e.normalizeCursorX(e.cursorX))
		}
	}()
	e.updateWindow()
}

func (e *editorImpl) updateWindow() {
	// Update the window atomically by replacing it. This is more efficient than multiple Print calls
	// on the user-visible window, which may result in flashes.
	windowY, windowX := e.window.YX()
	maxY, maxX := e.window.MaxYX()
	newWindow, _ := gc.NewWindow(maxY, maxX, windowY, windowX)
	newWindow.SetBackground(COLOR_PAIR_DEFAULT)
	for i := 0; i <= e.getMaxYForContent(); i++ {
		// We reserve the bottom 2 lines for user messages, and debug messages.
		if i+e.fileLineOffset < len(e.fileContents) {
			line := e.fileContents[e.fileLineOffset+i]
			newWindow.Println(line)
		} else if (e.verbose && i < maxY-2) || (!e.verbose && i < maxY-1) {
			// There are no more file contents, so use a special UI to denote that these lines are
			// not present in the file.
			// We need to reserve either 1 or 2 lines without this UI treatment. 1 if there is no
			// debug message, 2 otherwise.
			newWindow.AttrOn(gc.A_DIM)
			newWindow.Println("~")
			newWindow.AttrOff(gc.A_DIM)
		}
	}
	if e.verbose {
		// Print debug output.
		newWindow.ColorOn(COLOR_PAIR_DEBUG)
		newWindow.Print("DEBUG: ")
		newWindow.Printf("build=%s; ", build_version.GetVersion())
		newWindow.Printf("file len=%d lines; ", len(e.fileContents))
		newWindow.Printf("curr line len=%d chars; ", len(e.fileContents[e.getCurrLineInd()]))
		newWindow.Printf("curr line offset=%d lines; ", e.fileLineOffset)
		newWindow.Printf("cursor=(x=%d,y=%d); ", e.cursorX, e.cursorY)
		newWindow.Printf("mode=%s", e.mode)
		newWindow.Println()
		newWindow.ColorOff(COLOR_PAIR_DEBUG)
	} else {
		// Print a newline anyways so no shifts when user toggles verbosity.
		newWindow.Println()
	}
	newWindow.Println(e.userMsg)

	e.window.Erase()
	e.window.SetBackground(gc.ColorPair(COLOR_PAIR_DEFAULT))
	e.window.Overlay(newWindow)
}

func (e *editorImpl) normalizeCursorX(x int) int {
	// In INSERT mode, it's expected for the cursor to be equal to the length of the current line.
	if e.mode == NORMAL_MODE && e.cursorX >= len(e.fileContents[e.getCurrLineInd()]) {
		// Special handling of x-position. See moveCursorInternal for details.
		x = len(e.fileContents[e.getCurrLineInd()]) - 1
	}
	if x < 0 {
		x = 0
	}
	return x
}

func (e *editorImpl) getCurrLineInd() int {
	return e.fileLineOffset + e.cursorY
}

func (e *editorImpl) getMaxYForContent() int {
	maxY, _ := e.window.MaxYX()
	// We reserve the bottom 2 lines for debug and user messages. Then we subtract 1 more since this
	// is an offset.
	return maxY - 3
}

// Each string is the entire row. The row does NOT contain the ending newline.
func getFileContentsAndLen(file *os.File) ([]string, int) {
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
	return fileContents, len(contents)
}
