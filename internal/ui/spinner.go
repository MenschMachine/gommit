package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type Spinner struct {
	writer  io.Writer
	text    string
	stopCh  chan struct{}
	doneCh  chan struct{}
	enabled bool
}

func StartSpinner(writer io.Writer, text string) *Spinner {
	s := &Spinner{
		writer: writer,
		text:   text,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	if !isTerminal(writer) || os.Getenv("TERM") == "dumb" {
		fmt.Fprintf(writer, "%s...\n", text)
		return s
	}

	s.enabled = true
	go s.spin()
	return s
}

func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	close(s.stopCh)
	<-s.doneCh
	clearLen := len(s.text) + 2
	fmt.Fprintf(s.writer, "\r%s\r", strings.Repeat(" ", clearLen))
}

func (s *Spinner) spin() {
	defer close(s.doneCh)
	frames := []rune{'|', '/', '-', '\\'}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			fmt.Fprintf(s.writer, "\r%c %s", frames[i%len(frames)], s.text)
			i++
		}
	}
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
