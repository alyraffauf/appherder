package main

import (
	"fmt"
	"io"
)

type cliProgress struct {
	out io.Writer
}

func newCliProgress(out io.Writer) *cliProgress {
	return &cliProgress{out: out}
}

func (p *cliProgress) Download(name string, received, total int64) {
	if total > 0 {
		percent := received * 100 / total
		fmt.Fprintf(p.out, "\r%s: %s / %s (%d%%)", truncate(name), humanSize(received), humanSize(total), percent)
		return
	}
	fmt.Fprintf(p.out, "\r%s: %s", truncate(name), humanSize(received))
}

func (p *cliProgress) Done() {
	fmt.Fprint(p.out, "\r\033[K")
}

func truncate(name string) string {
	if len(name) > 20 {
		return name[:17] + "..."
	}
	return name
}
