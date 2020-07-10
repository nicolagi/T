package main

import (
	"bytes"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fhs/9fans-go/acme"
	"github.com/lionkov/go9p/p"
)

type windowMode int

const (
	modeHome         windowMode = iota // /twitter/home
	modeMentions                       // /twitter/mentions
	modeUpdate                         // /twitter/new or /twitter/reply/$tweet
	modeUserTimeline                   // /twitter/users/$user
)

var all struct {
	sync.Mutex
	m map[*acme.Win]*window
}

var decimalDigits = regexp.MustCompile("^[0-9]+$")

type window struct {
	*acme.Win

	mode windowMode

	screenName string // For modeUserTimeline
	inReply    string // For modeUpdate
}

// newWindow creates a window in acme without a specific purpose,
// and registers it in the global map of windows.
func newWindow(pathname string) *window {
	all.Lock()
	defer all.Unlock()
	if all.m == nil {
		all.m = make(map[*acme.Win]*window)
	}
	aw, err := acme.New()
	if err != nil {
		time.Sleep(10 * time.Millisecond)
		aw, err = acme.New()
		if err != nil {
			log.Fatal("Could not create acme window again")
		}
	}
	aw.SetErrorPrefix(pathname)
	_ = aw.Name(pathname)
	w := &window{Win: aw}
	all.m[w.Win] = w
	return w
}

// resetTag is used when a new window is created,
// or when transitioning a window from one mode to another.
func (w *window) resetTag() {
	var tag string
	switch w.mode {
	case modeUserTimeline, modeHome, modeMentions:
		tag = " New Reply Newer Older Trim Get "
	case modeUpdate:
		tag = " Post "
	}
	_ = w.Ctl("cleartag")
	_ = w.Fprintf("tag", tag)
}

func (w *window) load() {
	var buf bytes.Buffer
	var err error
	switch w.mode {
	case modeHome:
		err = printTimeline(&buf, "home")
	case modeMentions:
		err = printTimeline(&buf, "mentions")
	case modeUserTimeline:
		err = printTimeline(&buf, path.Join("users", w.screenName))
	}
	w.Clear()
	if err != nil {
		_, _ = w.Write("body", []byte(err.Error()))
	} else {
		_, _ = w.Write("body", buf.Bytes())
		_ = w.Ctl("clean")
	}
	_ = w.Ctl("dot=addr")
	_ = w.Ctl("show")
}

func (w *window) loop() {
	defer w.exit()
	w.EventLoop(w)
}

// exit is called after the window's event loop is over, i.e., the
// window has been closed in acme. If it's the last window, we
// terminate the process.
func (w *window) exit() {
	all.Lock()
	defer all.Unlock()
	if all.m[w.Win] == w {
		delete(all.m, w.Win)
	}
	if len(all.m) == 0 {
		fsys.Unmount()
		os.Exit(0)
	}
}

// Look implements acme.EventHandler.
func (w *window) Look(text string) (handled bool) {
	if decimalDigits.MatchString(text) {
		// TODO: Try to open a tweet window.
		return false
	} else {
		screenName := strings.ToLower(text)
		stat, err := fsys.FStat(path.Join("users", screenName))
		if err != nil {
			return false
		}
		if stat.Mode&p.DMDIR == 0 {
			return false
		}
		newUserTimelineWindow(screenName)
		return true
	}
}

// Execute implements acme.EventHandler.
func (w *window) Execute(command string) (handled bool) {
	switch w.mode {
	case modeHome, modeMentions, modeUserTimeline:
		switch command {
		case "Reply":
			idStr := w.Selection()
			if decimalDigits.MatchString(idStr) {
				newUpdateWindow(idStr)
			}
			return true
		case "New":
			newUpdateWindow("")
			return true
		case "Newer":
			if w.mode == modeUserTimeline {
				fsysCommand("newer @%s", w.screenName)
			} else if w.mode == modeHome {
				fsysCommand("newer home")
			} else if w.mode == modeMentions {
				fsysCommand("newer mentions")
			}
			w.load()
			return true
		case "Older":
			if w.mode == modeUserTimeline {
				fsysCommand("older @%s", w.screenName)
			} else if w.mode == modeHome {
				fsysCommand("older home")
			} else if w.mode == modeMentions {
				fsysCommand("older mentions")
			}
			w.load()
			return true
		case "Get":
			w.load()
			return true
		case "Trim":
			if w.mode == modeUserTimeline {
				fsysCommand("trim @%s 10", w.screenName)
			} else if w.mode == modeHome {
				fsysCommand("trim home 10")
			} else if w.mode == modeMentions {
				fsysCommand("trim mentions 10")
			}
			w.load()
			return true
		}
	case modeUpdate:
		switch command {
		case "Post":
			text, err := w.ReadAll("body")
			if err != nil {
				w.Errf("Failed reading body: %+v", err)
				return false
			}
			if w.inReply == "" {
				err = fsysCommand("post %s", text)
			} else {
				err = fsysCommand("reply %s %s", w.inReply, text)
			}
			if err != nil {
				w.Errf("Failed posting via fs: %+v", err)
				return false
			}
			_ = w.Del(true)
			return true
		}
	}
	return false
}

func newUserTimelineWindow(screenName string) {
	title := "/twitter/users/" + screenName
	if acme.Show(title) != nil {
		return
	}
	w := newWindow(title)
	w.mode = modeUserTimeline
	w.screenName = screenName
	w.resetTag()
	go w.load()
	go w.loop()
}

func newHomeWindow() {
	title := "/twitter/home"
	if acme.Show(title) != nil {
		return
	}
	w := newWindow(title)
	w.mode = modeHome
	w.resetTag()
	go w.load()
	go w.loop()
}

func newMentionsWindow() {
	title := "/twitter/mentions"
	if acme.Show(title) != nil {
		return
	}
	w := newWindow(title)
	w.mode = modeMentions
	w.resetTag()
	go w.load()
	go w.loop()
}

func newUpdateWindow(inReply string) {
	var title string
	if inReply == "" {
		title = "/twitter/new"
	} else {
		title = "/twitter/reply/" + inReply
	}
	if acme.Show(title) != nil {
		return
	}
	w := newWindow(title)
	w.mode = modeUpdate
	w.inReply = inReply
	w.resetTag()
	go w.loop()
}
