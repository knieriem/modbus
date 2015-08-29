package rtu

import (
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/knieriem/text/rc"
)

type cmd struct {
	*exec.Cmd
}

func parseCommand(spec string) (c *cmd, match bool) {
	if !strings.HasPrefix(spec, "!") {
		return
	}
	if len(spec) < 2 {
		return
	}
	match = true
	args := rc.Tokenize(spec[1:])
	c = new(cmd)
	c.Cmd = exec.Command(args[0], args[1:]...)
	return
}

func (c *cmd) Dial() (f io.ReadWriteCloser, err error) {
	w, err := c.StdinPipe()
	if err != nil {
		return
	}
	r, err := c.StdoutPipe()
	if err != nil {
		return
	}
	c.Stderr = os.Stderr
	err = c.Start()
	if err != nil {
		return
	}

	type conn struct {
		io.Reader
		io.WriteCloser
	}
	f = &conn{r, w}
	return

}
