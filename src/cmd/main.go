package main

import (
	"bytes"
	"github.com/pkg/term"
	"os"
	"os/signal"
	"syscall"
	"text_editor/internal"
)

func main() {
	// TODO(omar): Proper flag management. For now, arg 1 is the file name to edit (arg 0 is program
	// name always).
	editor, err := internal.NewEditor(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer editor.Close()

	t, err := term.Open(os.Stdin.Name())
	if err != nil {
		panic(err)
	}
	t.SetCbreak()
	defer t.Restore()

	// Also cleanup on process exit.
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)
		<-signalChan
		editor.Close()
		t.Restore()
		os.Exit(0)
	}()

	for {
		runeBytes := make([]byte, 1)
		_, err := os.Stdin.Read(runeBytes)
		if err != nil {
			break
		}
		chs := bytes.Runes(runeBytes)
		ch := chs[0]
		if ch == 'q' {
			break
		}
		err = editor.Handle(ch)
		if err != nil {
			panic(err)
		}
	}
}
