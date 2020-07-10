package main

import (
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/lionkov/go9p/p"
	"github.com/pkg/errors"
)

type byModified []*p.Dir

func (dirs byModified) Len() int { return len(dirs) }

func (dirs byModified) Less(a, b int) bool {
	return dirs[a].Mtime > dirs[b].Mtime
}

func (dirs byModified) Swap(a, b int) {
	dirs[a], dirs[b] = dirs[b], dirs[a]
}

func printTimeline(w io.Writer, timeline string) error {
	f, err := fsys.FOpen(timeline, p.OREAD)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()
	children, err := f.Readdir(0)
	if err != nil && err != io.EOF {
		return errors.WithStack(err)
	}
	sort.Sort(byModified(children))
	buf := make([]byte, 4096)
	for _, id := range children {
		tweet, err := fsys.FOpen(path.Join(timeline, id.Name), p.OREAD)
		if err != nil {
			tweet.Close()
			return errors.WithStack(err)
		}
		n, err := tweet.Read(buf)
		if err != nil && err != io.EOF {
			tweet.Close()
			return errors.WithStack(err)
		}
		fmt.Fprintf(w, "--- %s\n", id.Name)
		w.Write(buf[:n])
		tweet.Close()
	}
	return nil
}
