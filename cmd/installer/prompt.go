package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// prompter is a line-based dialogue so it works on any terminal, ssh included (master D9); done aborts every prompt with the empty answer.
type prompter struct {
	done  <-chan struct{}
	lines <-chan string
	out   io.Writer
}

// newPrompter reads lines in the background so a cancellation is seen while a prompt waits; the reader ends with its input.
func newPrompter(done <-chan struct{}, in io.Reader, out io.Writer) *prompter {
	lines := make(chan string)
	go readLines(bufio.NewReader(in), lines)
	return &prompter{done: done, lines: lines, out: out}
}

// ask answers empty at end of input or on done, so a closed stdin takes every default and an interrupt falls through.
func (p *prompter) ask(question string) string {
	fmt.Fprintf(p.out, "%s: ", question)
	select {
	case line := <-p.lines:
		return line
	case <-p.done:
		return ""
	}
}

// choose keeps fallback on Enter and asks again on an invalid answer.
func (p *prompter) choose(question string, choices []string, fallback int) int {
	for index, choice := range choices {
		fmt.Fprintf(p.out, "  %d) %s\n", index+1, choice)
	}
	for {
		answer := p.ask(fmt.Sprintf("%s [%d]", question, fallback+1))
		if answer == "" {
			return fallback
		}

		number, err := strconv.Atoi(answer)
		isValid := err == nil && number >= 1 && number <= len(choices)
		if isValid {
			return number - 1
		}
		fmt.Fprintf(p.out, "pick 1-%d\n", len(choices))
	}
}

func (p *prompter) confirm(question string) bool {
	answer := strings.ToLower(p.ask(question + " [y/N]"))
	return answer == "y" || answer == "yes"
}

// readLines sends every trimmed line of in and closes lines at end of input, so later reads answer empty.
func readLines(in *bufio.Reader, lines chan<- string) {
	defer close(lines)
	for {
		line, err := in.ReadString('\n')
		if line != "" || err == nil {
			lines <- strings.TrimSpace(line)
		}
		if err != nil {
			return
		}
	}
}
