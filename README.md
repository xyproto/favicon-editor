# Favicon Editor [![Go Report Card](https://goreportcard.com/badge/github.com/xyproto/favicon-editor)](https://goreportcard.com/report/github.com/xyproto/favicon-editor) [![License](https://img.shields.io/badge/license-BSD-green.svg?style=flat)](https://raw.githubusercontent.com/xyproto/favicon-editor/master/LICENSE)

View, create and edit `favicon.ico` and `favicon.png` images by using a TUI.

## Quick start

You can install `favicon-editor` with Go 1.17 or later:

    go install github.com/xyproto/favicon-editor@latest

## Features and limitations

* Can open both Icon files and PNG files.
* Will only save graphics as 16-color graysacle images.
* Lets you draw a simple `favicon.ico` file even if you are ssh'd into a server.

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

    git clone https://github.com/xyproto/favicon-editor
    cd favicon-editor
    go build -mod=vendor
    sudo install -Dm755 o /usr/bin/favicon-editor
    gzip fav.1
    sudo install -Dm644 favicon-editor.1.gz /usr/share/man/man1/favicon-editor.1.gz

## General info

* Version: 1.0.0
* License: 3-clause BSD
* Author: Alexander F. Rødseth &lt;xyproto@archlinux.org&gt;
