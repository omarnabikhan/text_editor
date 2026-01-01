# (WIP) Implementing Gim

This is a little diary/blog of implementing Gim, this text editor. As a fun challenge, I'm only going to edit this blog with Gim!

# Motivation

I've always liked Vim, I think it's a really cool way of editing text, specifically programs. I also like that it is very lightweight, and very fast. There
is no index to maintain, there is no build process happening, no LLM calls going on (in vanilla mode at least), which I find very nice. It's a clean
implementation. I also like Go, the programming language, so I'm combining both interests and developing a text editor.

# Status

It's Jan 1st 2026, and I've spent about a week on this project. I can use Gim for small text edits, and it has a basic subset of Vim commands. I can open
a file, write to it, move my cursor around without too many issues. I haven't gotten to implementing the more fun commands like stringing together the
different Vim verbs + nouns, nor syntax highlighting. Also, the efficiency in terms of file manipulation and text changes isn't very good (I just read the
entire file in memory, even if not all of it is viewable). But still, it's useable.
  
# Challenges

## Using ncurses

I wanted to use the most primitive tools available to me, so the TUI (Terminal UI) I'm using is ncurses, which is a TUI written in C. It's very basic, and
I can use it since Go has cgo (a way to call pure C from Go) and someone has graciously written a Go package goncurses to port it for me. Working with that
library has been an little difficult for a few reasons.
 1. The library is very old. It's initial release was in 1993 (older than me!), so it's a little bit dated.
 2. Documentation is probably good, but just really verbose. Nobody is making tutorials for how to use ncurses in 2026.
 3. Ncurses is super primitive and you can majorly mess up. When I was first getting the text to show on screen, I'd mess my terminal's color scheme and
    all _after_ the program exited.

Adding colors was the most painful so far. And there are really not too many colors in the current implementation, just a non-black background, and a
brighter white text color.

## Managing state

This is an implementation specific challenge. I didn't want to sit and think about the best implementation from the beginning, since there are so many
challenges in text editing, and I could probably sit and think about it forever. Rather, I just wanted to start by doing, and fix as I go. An engineering
mentor has a saying that "Make today better than yesterday," so it's OK if the code starts out super basic and inefficienct: it'll only get better.

A responsibility does come from that though: you've got to constantly be thinking of ways to improve. And as I add more features to Gim, there is an
obvious necessity to improve. With a small subset of features implemented, it's hard to say a particular implementation is much better than some other
implementation. But, as you expand the feature set, and have an idea of the direction you want to move in, then some implementations are obviously more
preferrable (and some are obviously less). So this has been guiding me as I implement Gim.

I've had a few iterations of how I manage state in the text editor, particularly the cursor position. I'll do a deep dive of this at some point, but the
main issue now is that I have 3 modes supported (Normal, Insert, Command). Each mode has a slightly different idea of how the cursor's position should be
modified and what values it may take on. For example, in Normal mode, the cursor can occupy a max x-position equal to the length of the current line - 1.
This is because it's an index in a string. But, in Insert mode, the cursor may actually be equal to the current length of the line, since you may add a
character at the end of the line.

So I had some refactors that made the state more manageable, and tried to take any mode specific things out of editor_impl.go, and keep those details
private to the specific mode impls (e.g. normal_mode_impl.go). This is also nice since each mode can keep private state too out of the way of other
modes (e.g. Command mode has a commandBuffer which no other mode needs to know about).
