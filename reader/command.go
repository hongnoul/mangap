package reader

import (
	"io"
	"os"

	"github.com/hongnoul/mangap/reader/pages"
)

// Command reads a chapter as a bubbletea ExecCommand (tea.Exec): the TUI
// releases the terminal, the reader owns it until q, then the TUI
// repaints. It satisfies tea.ExecCommand structurally, so this package
// does not import bubbletea.
type Command struct {
	src pages.Source
	opt Options

	// Result is where the reader stopped, valid after Run.
	Result Result
}

// NewCommand wraps src. Command.Run closes src.
func NewCommand(src pages.Source, opt Options) *Command {
	return &Command{src: src, opt: opt}
}

// Run reads until the user quits, then closes the source.
func (c *Command) Run() error {
	defer c.src.Close()
	res, err := Run(c.src, c.opt)
	c.Result = res
	return err
}

// SetStdin uses r when it is a terminal file, else the process stdin.
func (c *Command) SetStdin(r io.Reader) {
	if f, ok := r.(*os.File); ok {
		c.opt.In = f
	}
}

// SetStdout uses w when it is a terminal file, else the process stdout.
func (c *Command) SetStdout(w io.Writer) {
	if f, ok := w.(*os.File); ok {
		c.opt.Out = f
	}
}

// SetStderr is unused: the reader never writes to stderr except traces.
func (c *Command) SetStderr(io.Writer) {}
