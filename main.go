package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/xyproto/vt100"
)

const version = "favicon 1.0.0"

var (
	rebasePrefixes   = []string{"p", "pick", "r", "reword", "d", "drop", "e", "edit", "s", "squash", "f", "fixup", "x", "exec", "b", "break", "l", "label", "t", "reset", "m", "merge"}
	checkboxPrefixes = []string{"- [ ]", "- [x]", "- [X]", "* [ ]", "* [x]", "* [X]"}
)

func main() {
	var (
		// Color scheme for the "text edit" mode
		defaultEditorForeground      = vt100.LightGreen
		defaultEditorBackground      = vt100.BackgroundDefault
		defaultStatusForeground      = vt100.White
		defaultStatusBackground      = vt100.BackgroundBlack
		defaultStatusErrorForeground = vt100.LightRed
		defaultStatusErrorBackground = vt100.BackgroundDefault
		defaultEditorSearchHighlight = vt100.LightMagenta

		versionFlag = flag.Bool("version", false, "show version information")
		helpFlag    = flag.Bool("help", false, "show simple help")

		statusDuration = 2700 * time.Millisecond

		copyLine   string // for the cut/copy/paste functionality
		statusMode bool   // if information should be shown at the bottom

		locationHistory map[string]int // remember where we were in each absolute filename

		clearOnQuit bool // clear the terminal when quitting, or not

		spacesPerTab = 4 // default spaces per tab

		mode Mode // an "enum"/int signalling if this file should be in git mode, markdown mode etc
	)

	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		return
	}

	if *helpFlag {
		fmt.Println(version + " - simple and limited text editor")
		fmt.Print(`
Hotkeys

ctrl-q     to quit
ctrl-s     to save
ctrl-a     go to start of line, then start of text and then the previous line
ctrl-e     go to end of line and then the next line
ctrl-p     to scroll up 10 lines
ctrl-n     to scroll down 10 lines or go to the next match if a search is active
ctrl-k     to delete characters to the end of the line, then delete the line
ctrl-g     to toggle filename/line/column/unicode/word count status display
ctrl-d     to delete a single character
ctrl-x     to cut the current line
ctrl-c     to copy the current line
ctrl-v     to paste the current line
ctrl-u     to undo
ctrl-l     to jump to a specific line
esc        to redraw the screen and clear the last search
ctrl-space to export to the other image format
ctrl-~     to save and quit + clear the terminal

Set NO_COLOR=1 to disable colors.

`)
		return
	}

	filename, lineNumber := FilenameAndLineNumber(flag.Arg(0), flag.Arg(1))
	if filename == "" {
		fmt.Fprintln(os.Stderr, "Need a filename.")
		os.Exit(1)
	}

	// If the filename ends with "." and the file does not exist, assume this was an attempt at tab-completion gone wrong.
	// If there are multiple files that exist that start with the given filename, open the one first in the alphabet (.cpp before .o)
	if strings.HasSuffix(filename, ".") && !exists(filename) {
		// Glob
		matches, err := filepath.Glob(filename + "*")
		if err == nil && len(matches) > 0 { // no error and at least 1 match
			sort.Strings(matches)
			filename = matches[0]
		}
	}

	baseFilename := filepath.Base(filename)

	// Initialize the terminal
	tty, err := vt100.NewTTY()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
	defer tty.Close()
	vt100.Init()

	// Create a Canvas for drawing onto the terminal
	c := vt100.NewCanvas()
	c.ShowCursor()

	// scroll 10 lines at a time, no word wrap
	e := NewEditor(spacesPerTab, defaultEditorForeground, defaultEditorBackground, true, 10, defaultEditorSearchHighlight, mode)

	// Adjust the word wrap if the terminal is too narrow
	w := int(c.Width())
	if w < e.wordWrapAt {
		e.wordWrapAt = w
	}

	// Use a theme for light backgrounds if XTERM_VERSION is set,
	// because $COLORFGBG is "15;0" even though the background is white.
	xterm := os.Getenv("XTERM_VERSION") != ""
	if xterm {
		e.setLightTheme()
	}

	e.respectNoColorEnvironmentVariable()

	status := NewStatusBar(defaultStatusForeground, defaultStatusBackground, defaultStatusErrorForeground, defaultStatusErrorBackground, e, statusDuration)
	status.respectNoColorEnvironmentVariable()

	// Load a file, or a prepare an empty version of the file (without saving it until the user saves it)
	var (
		statusMessage  string
		warningMessage string
	)

	// We wish to redraw the canvas and reposition the cursor
	e.redraw = true
	e.redrawCursor = true

	// Use os.Stat to check if the file exists, and load the file if it does
	if fileInfo, err := os.Stat(filename); err == nil {

		// TODO: Enter file-rename mode when opening a directory?
		// Check if this is a directory
		if fileInfo.IsDir() {
			quitError(tty, errors.New(filename+" is a directory"))
		}

		warningMessage, err = e.Load(c, tty, filename)
		if err != nil {
			quitError(tty, err)
		}

		if !e.Empty() {
			statusMessage = "Loaded " + filename + warningMessage
		} else {
			statusMessage = "Loaded empty file: " + filename + warningMessage
		}

		// Test write, to check if the file can be written or not
		testfile, err := os.OpenFile(filename, os.O_WRONLY, 0664)
		if err != nil {
			// can not open the file for writing
			statusMessage += " (read only)"
			// set the color to red when in read-only mode
			e.fg = vt100.Red
			// do a full reset and redraw
			c = e.FullResetRedraw(c, status)
			// draw the editor lines again
			e.DrawLines(c, false, true)
			e.redraw = false
		}
		testfile.Close()
	} else {
		newMode, err := e.PrepareEmpty(c, tty, filename)
		if err != nil {
			quitError(tty, err)
		}

		statusMessage = "New " + filename

		// For .ico and .png
		if newMode != modeBlank {
			mode, e.mode = newMode, newMode
		}

		// Test save, to check if the file can be created and written, or not
		if err := e.Save(&filename, true, false); err != nil {
			// Check if the new file can be saved before the user starts working on the file.
			quitError(tty, err)
		} else {
			// Creating a new empty file worked out fine, don't save it until the user saves it
			if os.Remove(filename) != nil {
				// This should never happen
				quitError(tty, errors.New("could not remove an empty file that was just created: "+filename))
			}
		}
	}

	// The editing mode is decided at this point

	// Undo buffer with room for 8192 actions
	undo := NewUndo(8192)

	// Resize handler
	SetUpResizeHandler(c, e, status, tty)

	tty.SetTimeout(2 * time.Millisecond)

	previousX := 1
	previousY := 1

	// Find the absolute path to this filename
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		absFilename = filename
	}

	// Load the location history, if available
	locationHistory = LoadLocationHistory(expandUser(locationHistoryFilename))

	// Check if a line number was given on the command line
	if lineNumber > 0 {
		e.GoToLineNumber(lineNumber, c, status, false)
		e.redraw = true
		e.redrawCursor = true
	} else if recordedLineNumber, ok := locationHistory[absFilename]; ok {
		// If this filename exists in the location history, jump there
		lineNumber = recordedLineNumber
		e.GoToLineNumber(lineNumber, c, status, true)
		e.redraw = true
		e.redrawCursor = true
	} else {
		// Draw editor lines from line 0 to h onto the canvas at 0,0
		e.DrawLines(c, false, false)
		e.redraw = false
	}

	if e.redraw {
		e.Center(c)
		e.DrawLines(c, true, false)
		e.redraw = false
	}

	status.SetMessage(statusMessage)
	status.Show(c, e)

	if e.redrawCursor {
		x := e.pos.ScreenX()
		y := e.pos.ScreenY()
		previousX = x
		previousY = y
		vt100.SetXY(uint(x), uint(y))
		e.redrawCursor = false
	}

	var (
		quit        bool
		previousKey string
	)

	for !quit {
		key := tty.String()
		switch key {
		case "c:17": // ctrl-q, quit
			quit = true
		case "c:0": // ctrl-space, build source code to executable, word wrap, convert to PDF or write to PNG, depending on the mode
			if strings.HasSuffix(baseFilename, ".ico") {
				// Save .ico as .png
				err := e.Save(&filename, !e.DrawMode(), true)
				if err != nil {
					statusMessage = err.Error()
					status.ClearAll(c)
					status.SetMessage(statusMessage)
					status.Show(c, e)
				} else {
					status.ClearAll(c)
					status.SetMessage("Saved " + strings.Replace(baseFilename, ".ico", ".png", 1))
					status.Show(c, e)
				}
				break // from case
			} else if strings.HasSuffix(baseFilename, ".png") {
				// Save .png as .ico
				err := e.Save(&filename, !e.DrawMode(), true)
				if err != nil {
					statusMessage = err.Error()
					status.ClearAll(c)
					status.SetMessage(statusMessage)
					status.Show(c, e)
				} else {
					status.ClearAll(c)
					status.SetMessage("Saved " + strings.Replace(baseFilename, ".png", ".ico", 1))
					status.Show(c, e)
				}
				break // from case
			}
			// Building this file extension is not implemented yet.
			status.ClearAll(c)
			// Just display the current time and word count.
			statusMessage := fmt.Sprintf("%d words, %s", e.WordCount(), time.Now().Format("15:04")) // HH:MM
			status.SetMessage(statusMessage)
			status.Show(c, e)
		case "←": // left arrow
			if !e.DrawMode() {
				e.Prev(c)
				if e.AfterLineScreenContents() {
					e.End()
				}
				e.SaveX(true)
			} else {
				// Draw mode
				e.pos.Left()
			}
			e.redrawCursor = true
		case "→": // right arrow
			if !e.DrawMode() {
				if e.DataY() < e.Len() {
					e.Next(c)
				}
				if e.AfterLineScreenContents() {
					e.End()
				}
				e.SaveX(true)
			} else {
				// Draw mode
				e.pos.Right(c)
			}
			e.redrawCursor = true
		case "↑": // up arrow
			// Move the screen cursor
			if !e.DrawMode() {
				if e.DataY() > 0 {
					// Move the position up in the current screen
					if e.UpEnd(c) != nil {
						// If below the top, scroll the contents up
						if e.DataY() > 0 {
							e.redraw = e.ScrollUp(c, status, 1)
							e.redrawCursor = true
							e.pos.Down(c)
							e.UpEnd(c)
						}
					}
					// If the cursor is after the length of the current line, move it to the end of the current line
					if e.AfterLineScreenContents() {
						e.End()
					}
				}
				// If the cursor is after the length of the current line, move it to the end of the current line
				if e.AfterLineScreenContents() {
					e.End()
				}
			} else {
				e.pos.Up()
			}
			e.redrawCursor = true
		case "↓": // down arrow
			if !e.DrawMode() {
				if e.DataY() < e.Len() {
					// Move the position down in the current screen
					if e.DownEnd(c) != nil {
						// If at the bottom, don't move down, but scroll the contents
						// Output a helpful message
						if !e.AfterEndOfDocument() {
							e.redraw = e.ScrollDown(c, status, 1)
							e.redrawCursor = true
							e.pos.Up()
							e.DownEnd(c)
						}
					}
					// If the cursor is after the length of the current line, move it to the end of the current line
					if e.AfterLineScreenContents() {
						e.End()
					}
				}
				// If the cursor is after the length of the current line, move it to the end of the current line
				if e.AfterLineScreenContents() {
					e.End()
				}
			} else {
				e.pos.Down(c)
			}
			e.redrawCursor = true
		case "c:14": // ctrl-n, scroll down or jump to next match
			// Scroll down
			e.redraw = e.ScrollDown(c, status, e.pos.scrollSpeed)
			// If e.redraw is false, the end of file is reached
			if !e.redraw {
				status.Clear(c)
				status.SetMessage("EOF")
				status.Show(c, e)
			}
			e.redrawCursor = true
			if !e.DrawMode() && e.AfterLineScreenContents() {
				e.End()
			}
		case "c:16": // ctrl-p, scroll up
			e.redraw = e.ScrollUp(c, status, e.pos.scrollSpeed)
			e.redrawCursor = true
			if !e.DrawMode() && e.AfterLineScreenContents() {
				e.End()
			}
		case "c:27": // esc, clear search term, reset, clean and redraw
			c = e.FullResetRedraw(c, status)
		case " ": // space
			undo.Snapshot(e)
			// Place a space
			if !e.DrawMode() {
				e.InsertRune(c, ' ')
				e.redraw = true
			} else {
				e.SetRune(' ')
			}
			e.WriteRune(c)
			if e.DrawMode() {
				e.redraw = true
			} else {
				// Move to the next position
				e.Next(c)
			}
		case "c:13": // return
			undo.Snapshot(e)
			// if the current line is empty, insert a blank line
			if !e.DrawMode() {
				e.TrimRight(e.DataY())
				lineContents := e.CurrentLine()
				if e.pos.AtStartOfLine() {
					// Insert a new line a the current y position, then shift the rest down.
					e.InsertLineAbove()
					// Also move the cursor to the start, since it's now on a new blank line.
					e.pos.Down(c)
					e.Home()
				} else if e.AtOrBeforeStartOfTextLine() {
					x := e.pos.ScreenX()
					// Insert a new line a the current y position, then shift the rest down.
					e.InsertLineAbove()
					// Also move the cursor to the start, since it's now on a new blank line.
					e.pos.Down(c)
					e.pos.SetX(x)
				} else if e.AtOrAfterEndOfLine() && e.AtLastLineOfDocument() {
					leadingWhitespace := e.LeadingWhitespace()
					if len(lineContents) > 0 && (strings.HasSuffix(lineContents, "(") || strings.HasSuffix(lineContents, "{") || strings.HasSuffix(lineContents, "[")) {
						// "smart indentation"
						leadingWhitespace += "\t"
					}
					e.InsertLineBelow()
					h := int(c.Height())
					if e.pos.sy >= (h - 1) {
						e.ScrollDown(c, status, 1)
						e.redrawCursor = true
					}
					e.pos.Down(c)
					e.Home()
					// Insert the same leading whitespace for the new line, while moving to the right
					for _, r := range leadingWhitespace {
						e.InsertRune(c, r)
						e.Next(c)
					}
				} else if e.AfterEndOfLine() {
					leadingWhitespace := e.LeadingWhitespace()
					if len(lineContents) > 0 && (strings.HasSuffix(lineContents, "(") || strings.HasSuffix(lineContents, "{") || strings.HasSuffix(lineContents, "[")) {
						// "smart indentation"
						leadingWhitespace += "\t"
					}
					e.InsertLineBelow()
					e.Down(c, status)
					e.Home()
					// Insert the same leading whitespace for the new line, while moving to the right
					for _, r := range leadingWhitespace {
						e.InsertRune(c, r)
						e.Next(c)
					}
				} else {
					// Split the current line in two
					if !e.SplitLine() {
						// Grab the leading whitespace from the current line
						leadingWhitespace := e.LeadingWhitespace()
						// Insert a line below, then move down and to the start of it
						e.InsertLineBelow()
						e.Down(c, status)
						e.Home()
						// Insert the same leading whitespace for the new line, while moving to the right
						for _, r := range leadingWhitespace {
							e.InsertRune(c, r)
							e.Next(c)
						}
					} else {
						e.Down(c, status)
						e.Home()
					}
				}
			} else {
				if e.AtLastLineOfDocument() {
					e.CreateLineIfMissing(e.DataY() + 1)
				}
				e.pos.Down(c)
			}
			e.redraw = true
		case "c:8", "c:127": // ctrl-h or backspace
			undo.Snapshot(e)
			if !e.DrawMode() && e.EmptyLine() {
				e.DeleteLine(e.DataY())
				e.pos.Up()
				e.TrimRight(e.DataY())
				e.End()
			} else if !e.DrawMode() && e.pos.AtStartOfLine() {
				if e.DataY() > 0 {
					e.pos.Up()
					e.End()
					e.TrimRight(e.DataY())
					e.Delete()
				}
			} else {
				// Move back
				e.Prev(c)
				// Type a blank
				e.SetRune(' ')
				e.WriteRune(c)
				if !e.DrawMode() && !e.AtOrAfterEndOfLine() {
					// Delete the blank
					e.Delete()
				}
			}
			e.redrawCursor = true
			e.redraw = true
		case "c:9": // tab
			undo.Snapshot(e)
			if !e.DrawMode() {
				// Place a tab
				if !e.DrawMode() {
					e.InsertRune(c, '\t')
				} else {
					e.SetRune('\t')
				}
				// Write the spaces that represent the tab
				e.WriteTab(c)
				// Move to the next position
				if !e.DrawMode() {
					e.Next(c)
				}
			}
			e.redrawCursor = true
			e.redraw = true
		case "c:1", "c:25": // ctrl-a, home (or ctrl-y for scrolling up in the st terminal)
			// First check if we just moved to this line with the arrow keys
			justMovedUpOrDown := previousKey == "↓" || previousKey == "↑"
			// If at an empty line, go up one line
			if !justMovedUpOrDown && e.EmptyRightTrimmedLine() {
				e.Up(c, status)
				//e.GoToStartOfTextLine()
				e.End()
			} else if x, err := e.DataX(); err == nil && x == 0 && !justMovedUpOrDown {
				// If at the start of the line,
				// go to the end of the previous line
				e.Up(c, status)
				e.End()
			} else if e.AtStartOfTextLine() {
				// If at the start of the text, go to the start of the line
				e.Home()
			} else {
				// If none of the above, go to the start of the text
				e.GoToStartOfTextLine()
			}
			e.redrawCursor = true
			e.SaveX(true)
		case "c:5": // ctrl-e, end
			// First check if we just moved to this line with the arrow keys
			justMovedUpOrDown := previousKey == "↓" || previousKey == "↑"
			// If we didn't just move here, and are at the end of the line,
			// move down one line and to the end, if not,
			// just move to the end.
			if !justMovedUpOrDown && e.AfterEndOfLine() {
				e.Down(c, status)
				e.End()
			} else {
				e.End()
			}
			e.redrawCursor = true
			e.SaveX(true)
		case "c:4": // ctrl-d, delete
			undo.Snapshot(e)
			if e.Empty() {
				status.SetMessage("Empty")
				status.Show(c, e)
			} else {
				e.Delete()
				e.redraw = true
			}
			e.redrawCursor = true
		case "c:30": // ctrl-~, save and quit + clear the terminal
			clearOnQuit = true
			quit = true
			fallthrough
		case "c:19": // ctrl-s, save
			status.ClearAll(c)
			// Save the file
			if err := e.Save(&filename, !e.DrawMode(), false); err != nil {
				status.SetMessage(err.Error())
				status.Show(c, e)
			} else {
				// TODO: Go to the end of the document at this point, if needed
				// Lines may be trimmed for whitespace, so move to the end, if needed
				if !e.DrawMode() && e.AfterLineScreenContents() {
					e.End()
				}
				// Save the current location in the location history and write it to file
				e.SaveLocation(absFilename, locationHistory)
				// Status message
				status.SetMessage("Saved " + filename)
				status.Show(c, e)
				c.Draw()
			}
		case "c:21", "c:26": // ctrl-u or ctrl-z, undo (ctrl-z may background the application)
			if err := undo.Restore(e); err == nil {
				//c.Draw()
				x := e.pos.ScreenX()
				y := e.pos.ScreenY()
				vt100.SetXY(uint(x), uint(y))
				e.redrawCursor = true
				e.redraw = true
			} else {
				status.SetMessage("No more to undo")
				status.Show(c, e)
			}
		case "c:12": // ctrl-l, go to line number
			status.ClearAll(c)
			status.SetMessage("Go to line number:")
			status.ShowNoTimeout(c, e)
			lns := ""
			doneCollectingDigits := false
			for !doneCollectingDigits {
				numkey := tty.String()
				switch numkey {
				case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9": // 0 .. 9
					lns += numkey // string('0' + (numkey - 48))
					status.SetMessage("Go to line number: " + lns)
					status.ShowNoTimeout(c, e)
				case "c:8", "c:127": // ctrl-h or backspace
					if len(lns) > 0 {
						lns = lns[:len(lns)-1]
						status.SetMessage("Go to line number: " + lns)
						status.ShowNoTimeout(c, e)
					}
				case "c:27", "c:17": // esc or ctrl-q
					lns = ""
					fallthrough
				case "c:13": // return
					doneCollectingDigits = true
				}
			}
			status.ClearAll(c)
			if lns != "" {
				if ln, err := strconv.Atoi(lns); err == nil { // no error
					e.redraw = e.GoToLineNumber(ln, c, status, true)
				}
			}
			e.redrawCursor = true
		case "c:11": // ctrl-k, delete to end of line
			undo.Snapshot(e)
			if e.Empty() {
				status.SetMessage("Empty")
				status.Show(c, e)
			} else {
				e.DeleteRestOfLine()
				if !e.DrawMode() && e.EmptyRightTrimmedLine() {
					// Deleting the rest of the line cleared this line,
					// so just remove it.
					e.DeleteLine(e.DataY())
					// Then go to the end of the line, if needed
					if e.AtOrAfterEndOfLine() {
						e.End()
					}
				}
				vt100.Do("Erase End of Line")
				e.redraw = true
			}
			e.redrawCursor = true
		case "c:24": // ctrl-x, cut line
			undo.Snapshot(e)
			y := e.DataY()
			copyLine = e.Line(y)
			// Copy the line to the clipboard
			_ = clipboard.WriteAll(copyLine)
			e.DeleteLine(y)
			e.redrawCursor = true
			e.redraw = true
		case "c:3": // ctrl-c, copy the stripped contents of the current line
			trimmed := strings.TrimSpace(e.Line(e.DataY()))
			if trimmed != "" {
				copyLine = trimmed
				// Copy the line to the clipboard
				_ = clipboard.WriteAll(copyLine)
			}
			e.redrawCursor = true
			e.redraw = true
		case "c:22": // ctrl-v, paste
			undo.Snapshot(e)
			// Try fetching the line from the clipboard first
			lines, err := clipboard.ReadAll()
			if err == nil { // no error
				if strings.Contains(lines, "\n") {
					copyLine = strings.SplitN(lines, "\n", 2)[0]
				} else {
					copyLine = lines
				}
			}
			// Fix nonbreaking spaces
			copyLine = strings.Replace(copyLine, string([]byte{0xc2, 0xa0}), string([]byte{0x20}), -1)
			if e.EmptyRightTrimmedLine() {
				// If the line is empty, use the existing indentation before pasting
				e.SetLine(e.DataY(), e.LeadingWhitespace()+strings.TrimSpace(copyLine))
			} else {
				// If the line is not empty, insert the trimmed string
				e.InsertString(c, strings.TrimSpace(copyLine))
			}
			// Prepare to redraw the text
			e.redrawCursor = true
			e.redraw = true
		default:
			if len([]rune(key)) > 0 && unicode.IsLetter([]rune(key)[0]) { // letter
				undo.Snapshot(e)
				// Type the letter that was pressed
				if len([]rune(key)) > 0 {
					if !e.DrawMode() {
						// Was this a special case of "OK" as the first thing written?
						if key == "K" {
							e.InsertRune(c, 'O')
							e.WriteRune(c)
							e.Next(c)
						}
						// Insert a letter. This is what normally happens.
						e.InsertRune(c, []rune(key)[0])
						e.WriteRune(c)
						e.Next(c)
					} else {
						// Replace this letter.
						e.SetRune([]rune(key)[0])
						e.WriteRune(c)
					}
					e.redraw = true
				}
			} else if len([]rune(key)) > 0 && unicode.IsGraphic([]rune(key)[0]) { // any other key that can be drawn
				undo.Snapshot(e)

				// Place *something*
				r := []rune(key)[0]

				// "smart dedent"
				if r == '}' || r == ']' || r == ')' {
					lineContents := strings.TrimSpace(e.CurrentLine())
					whitespaceInFront := e.LeadingWhitespace()
					if e.pos.sx > 0 && len(lineContents) == 0 && len(whitespaceInFront) > 0 {
						// move one step left
						e.Prev(c)
						// trim trailing whitespace
						e.TrimRight(e.DataY())
					}
				}

				if !e.DrawMode() {
					e.InsertRune(c, []rune(key)[0])
				} else {
					e.SetRune([]rune(key)[0])
				}
				e.WriteRune(c)
				if len(string(r)) > 0 {
					if !e.DrawMode() {
						// Move to the next position
						e.Next(c)
					}
				}
				e.redrawCursor = true
				e.redraw = true
			}
		}
		previousKey = key
		// Redraw, if needed
		if e.redraw {
			// Draw the editor lines on the canvas, respecting the offset
			e.DrawLines(c, true, false)
			e.redraw = false
		} else if e.Changed() {
			c.Draw()
		}
		// Drawing status messages should come after redrawing, but before cursor positioning
		if statusMode {
			status.ShowLineColWordCount(c, e, filename)
		} else if status.isError {
			// Show the status message
			status.Show(c, e)
		}
		// Position the cursor
		x := e.pos.ScreenX()
		y := e.pos.ScreenY()
		if e.redrawCursor || x != previousX || y != previousY {
			vt100.SetXY(uint(x), uint(y))
			e.redrawCursor = false
		}
		previousX = x
		previousY = y
	}
	// Save the current location in the location history and write it to file
	e.SaveLocation(absFilename, locationHistory)

	// Clear all status bar messages
	status.ClearAll(c)

	// Quit everything that has to do with the terminal
	if clearOnQuit {
		vt100.Clear()
		vt100.Close()
	} else {
		c.Draw()
		fmt.Println()
	}
}
