package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	gc "github.com/gbin/goncurses"

	"text_editor/internal"
)

func main() {
	filePath := ""
	verbose := false
	help := false

	flag.StringVar(&filePath, "f", "", "which file to open")
	flag.BoolVar(&help, "h", false, "show usage and exit")
	flag.BoolVar(&verbose, "v", false, "enter in verbose mode (optional)")
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}

	if len(filePath) == 0 {
		fmt.Println("file must be provided via -f flag")
		flag.Usage()
		os.Exit(1)
	}

	window, _ := gc.Init()
	defer gc.End()

	gc.Echo(false)
	gc.CBreak(true)
	gc.StartColor()

	editor, err := internal.NewEditor(window, filePath, verbose)
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
