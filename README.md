# favicon [![Build Status](https://travis-ci.com/xyproto/favicon.svg?branch=master)](https://travis-ci.com/xyproto/o) [![Go Report Card](https://goreportcard.com/badge/github.com/xyproto/o)](https://goreportcard.com/report/github.com/xyproto/o) [![License](https://img.shields.io/badge/license-BSD-green.svg?style=flat)](https://raw.githubusercontent.com/xyproto/o/master/LICENSE)

`o` is a small and fast text editor that is limited to the VT100 standard.

It's a good fit for writing git commit messages, using `EDITOR=o git commit`.

For a more feature complete editor that is also written in Go, check out [micro](https://github.com/zyedidia/micro).

## Packaging status

[![Packaging status](https://repology.org/badge/vertical-allrepos/favicon.svg)](https://repology.org/project/favicon/versions)

## Quick start

You can install `favicon` with Go 1.10 or later:

    go get -u github.com/xyproto/favicon

## Features and limitations

* Edit `favicon.ico` and `favicon.png` images.
* Will only save 16-color graysacle images.

## Hotkeys

* `ctrl-q` - Quit
* `ctrl-s` - Save
* `ctrl-a` - Go to start of text, then start of line and then to the previous line.
* `ctrl-e` - Go to end of line and then to the next line.
* `ctrl-p` - Scroll up 10 lines.
* `ctrl-n` - Scroll down 10 lines, or go to the next match if a search is active.
* `ctrl-k` - Delete characters to the end of the line, then delete the line.
* `ctrl-d` - Delete a single character.
* `ctrl-x` - Cut the current line.
* `ctrl-c` - Copy the current line.
* `ctrl-v` - Paste the current line.
* `ctrl-u` - Undo (`ctrl-z` is also possible, but may background the application).
* `ctrl-l` - Jump to a specific line number.
* `esc` - Redraw the screen and clear the last search.
* `ctrl-space` - Export to `.png` if editing an `.ico` file. Export to `.ico` if editing a `.png` file.
* `ctrl-~` - Save and quit.

## Manual installation

On Linux:

    git clone https://github.com/xyproto/favicon
    cd favicon
    go build -mod=vendor
    sudo install -Dm755 o /usr/bin/favicon
    gzip favicon.1
    sudo install -Dm644 favicon.1.gz /usr/share/man/man1/favicon.1.gz

## General info

* Version: 1.0.0
* License: 3-clause BSD
* Author: Alexander F. Rødseth &lt;xyproto@archlinux.org&gt;
