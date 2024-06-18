package metrics

import (
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
)

type counter struct {
	name, description string
	counter           uint64
}

func (c *counter) Inc() {
	atomic.AddUint64(&c.counter, 1)
}

func (c *counter) Add(n uint64) {
	atomic.AddUint64(&c.counter, n)
}

func (c *counter) Print(w io.Writer) error {
	if c.description != "" {
		_, err := fmt.Fprintf(w, "# HELP %s %s\n", c.name, c.description)
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "# TYPE %s counter\n%s{} %s\n", c.name, c.name, strconv.FormatUint(atomic.LoadUint64(&c.counter), 10))
	return err
}

type printer interface {
	Print(w io.Writer) error
}

var (
	Errors     = &counter{name: "errors", description: "number of errors since the service was launched. see logs for details", counter: 0}
	Warnings   = &counter{name: "warnings", description: "number of warnings since the service was launched. see logs for details", counter: 0}
	Info       = &counter{name: "info", description: "number of infos since the service was launched. see logs for details", counter: 0}
	AllMetrics = []printer{Errors, Warnings, Info}
)
