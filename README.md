Gim
--
Gim is a Vim implementation in Go.

Installation
--

```sh
$ go install github.com/omarnabikhan/gim
```

Now, you can just use it however you'd use vim.
```sh
$ gim someFile
```

Development
--
Assuming you've cloned this repo and are in the root directory, run the `build_dev.sh` script to build the binary locally and add to your PATH.
Be sure that your GOBIN env var (usually`~/go/bin`) is in your PATH already, since this just runs `$ go install .` under the hood which copies
the executable to GOBIN
```
$ ./build_dev.sh
```

Motivation
--
Just for fun. I like Vim a lot, and text editors in general are fasincating to me. So I'm experimenting with implementing my own here, purely in Go.

Here's a fun update: I can now reasonably edit, save, and work with this README all in my editor built from scratch!

I initially started out using basic STDIN and STDOUT manipulation, but quickly moved into a more primitive direction. This binary just uses ncurses, which
is a very primitive library (written in C) to make TUI (terminal UIs). I'm using a Go port of it (and under the hood, that just uses CGO to actually call
the raw C library calls) so it's pretty efficient.

More and more, I get confused on if my program is running, or if Vim is running. So far, this project has just been me implementing VIM features, but I'm
going to challenge myself to come up with features not in Vim that I think are useful and implement them.

The most painful thing so far has been using Colors.

This README has become more of a blog. Maybe I'll update it in a future commit, but for now here it is.

Status
--
I've implemented the basic functionality. Now, there are 3 main (large) projects to do:
1. Implement proper Vimish. I've realized my Vimish is not very good, so I'm reading https://irian.to/blogs/mastering-vim-grammar to get better.
2. Syntax highlighting. Right now, the text is just white on grey.
3. Optimizations. This is a never-ending project, of course, and I'm not too worried about the performance for now. So this comes last.
