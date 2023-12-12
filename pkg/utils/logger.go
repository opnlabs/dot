package utils

import (
	"io"
	"sync"

	"github.com/fatih/color"
)

var colors = []color.Attribute{color.FgYellow, color.FgGreen, color.FgRed, color.FgWhite, color.FgMagenta}
var index = -1

var l sync.Mutex

const MaxNameLength = 20

// ColorLogger provides an io.Writer that can output in color.
type ColorLogger struct {
	name   string
	writer io.Writer
	c      color.Attribute
}

func NewColorLogger(name string, writer io.Writer, newColor bool) io.Writer {
	if newColor {
		l.Lock()
		defer l.Unlock()
		index = (index + 1) % len(colors)
	}

	if len(name) > MaxNameLength {
		name = name[:MaxNameLength-3] + "..."
	}

	return &ColorLogger{
		name:   name,
		writer: writer,
		c:      colors[index],
	}
}

func (c *ColorLogger) Write(p []byte) (int, error) {
	out := color.New(c.c)
	out.Print(c.name, " | ")
	return out.Fprintf(c.writer, "%s", p)
}
