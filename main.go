package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/pkg/errors"
)

var (
	network string
	address string

	fsys *clnt.Clnt
)

func fsysCommand(format string, a ...interface{}) error {
	f, err := fsys.FOpen("ctl", p.OWRITE)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()
	_, err = f.Write([]byte(fmt.Sprintf(format, a...)))
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type p9user struct{}

func (p9user) Id() int {
	return 0
}

func (p9user) Name() string {
	return os.Getenv("user")
}

func (p9user) IsMember(g p.Group) bool {
	return false
}

func (p9user) Groups() []p.Group {
	return nil
}

func main() {
	flag.StringVar(&network, "net", "tcp", "`network`, e.g., tcp or udp")
	flag.StringVar(&address, "addr", "127.0.0.1:7731", "`address`")
	flag.Parse()
	var user p.User
	if runtime.GOOS == "plan9" {
		user = &p9user{}
	} else {
		user = p.OsUsers.Uid2User(os.Getuid())
	}
	var err error
	fsys, err = clnt.Mount(network, address, user.Name(), 8192, user)
	if err != nil {
		log.Fatal(err)
	}
	newHomeWindow()
	newMentionsWindow()
	// Park this goroutine indefinitely. Program will exit when last
	// acme window owned by this process is deleted.
	select {}
}
