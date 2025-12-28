package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	gc "github.com/gbin/goncurses"

	"text_editor/internal"
)

func main() {
	verbose := flag.Bool("v", false, "enter in verbose mode (show debug data)")
	filePath := flag.String("f", "", "which file to open")
	flag.Parse()

	window, _ := gc.Init()
	defer gc.End()

	gc.Echo(false)
	gc.CBreak(true)
	gc.StartColor()

	editor, err := internal.NewEditor(window, *filePath, *verbose)
	if err != nil {
		panic(err)
	}
	defer editor.Close()

	// Also cleanup on process exit.
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)
		<-signalChan
		editor.Close()
		gc.End()
		os.Exit(0)
	}()

	for {
		key := window.GetChar()
		if gc.KeyString(key) == "q" {
			break
		}
		err = editor.Handle(key)
		if err != nil {
			panic(err)
		}
	}
}
