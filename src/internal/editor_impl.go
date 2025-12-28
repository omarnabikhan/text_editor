package internal

import (
	"fmt"
	"io"
	"os"
	src "text_editor"
)

const (
	// rw-rw-rw-
	cReadWriteFileMode = 0666

	// TODO(omar): For now, assumes the view is a constant of 100x10.
	cMaxWidth  = 100
	cMaxHeight = 10

	cColorHighlight  = "\u001b[32m"
	cColorReset      = "\u001b[0m"
	cAnsiClearScreen = "\033c"
)

func NewEditor(filePath string) (src.Editor, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, cReadWriteFileMode)
	if err != nil {
		return nil, err
	}
	e := &editorImpl{file: file}
	// Initial update of window.
	e.sync()
	return e, nil
}

type editorImpl struct {
	file *os.File

	// Mutable state.
	cursor  location
	verbose bool
}

var _ src.Editor = (*editorImpl)(nil)

func (e *editorImpl) Handle(ch rune) error {
	switch ch {
	case 'a':
		if e.cursor.x == 0 {
			break
		}
		e.cursor.x--
	case 's':
		// TODO(omar): move window if past max height
		e.cursor.y++
	case 'd':
		if e.cursor.x >= cMaxWidth {
			break
		}
		e.cursor.x++
	case 'w':
		if e.cursor.y == 0 {
			break
		}
		e.cursor.y--
	case 'v':
		e.verbose = !e.verbose
	}
	e.sync()
	return nil
}

func (e *editorImpl) Close() {
	e.file.Close()
}

func (e *editorImpl) sync() {
	defer os.Stdout.Sync()
	// Will clear the STDOUT file and write whatever is viewable.
	e.clearDisplay()
	e.updateWindow()
	//e.updateWindowStub()
}

func (e *editorImpl) clearDisplay() {
	os.Stdout.Write([]byte(cAnsiClearScreen))
}

func (e *editorImpl) updateWindowStub() {
	os.Stdout.WriteString("dummy")
}

func (e *editorImpl) updateWindow() {
	contents := e.getFileContents()
	bytes := []byte{}
	for row := 0; row < len(contents); row++ {
		for col := 0; col < len(contents[row]); col++ {
			highlightText := row == e.cursor.y && col == e.cursor.x
			if highlightText {
				bytes = append(bytes, []byte(cColorHighlight)...)
			}
			bytes = append(bytes, byte(contents[row][col]))
			if highlightText {
				bytes = append(bytes, []byte(cColorReset)...)
			}
		}
		bytes = append(bytes, '\n')
	}
	if e.verbose {
		// Print debug output.
		bytes = append(bytes, []byte("DEBUG:")...)
		bytes = append(bytes, []byte(fmt.Sprintf("contents is length %d", len(contents)))...)
		for _, line := range contents {
			bytes = append(bytes, []byte(string(line))...)
			bytes = append(bytes, '\n')
		}
	}
	_, err := os.Stdout.Write(bytes)
	if err != nil {
		panic(err)
	}
}

func (e *editorImpl) getFileContents() [][]rune {
	// Make sure file is being read from beginning.
	e.file.Seek(0 /*offset*/, io.SeekStart)
	contents, err := io.ReadAll(e.file)
	if err != nil {
		panic(err)
	}

	fileContents := make([][]rune, cMaxHeight)
	currRowInd := 0
	currRow := []rune{}
	for _, b := range contents {
		if b == '\n' {
			// Line break, meaning we update a new row.
			fileContents[currRowInd] = currRow
			currRowInd++
			currRow = nil
		} else {
			currRow = append(currRow, rune(b))
		}
	}
	return fileContents
}
