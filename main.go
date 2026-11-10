// Command ggraph shows an interactive git commit graph in the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iysun/ggraph/internal/gitlog"
	"github.com/iysun/ggraph/internal/ui"
)

func main() {
	var (
		dir     string
		maxSeen int
		all     bool
		dump    bool
		noColor bool
	)
	flag.StringVar(&dir, "C", ".", "run as if git was started in this directory")
	flag.IntVar(&maxSeen, "n", 300, "maximum number of commits (0 = all)")
	flag.BoolVar(&all, "all", true, "include commits from all refs")
	flag.BoolVar(&dump, "dump", false, "print the graph and exit instead of starting the TUI")
	flag.BoolVar(&noColor, "no-color", false, "disable color in --dump output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ggraph [flags]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg := ui.Config{Dir: dir, Max: maxSeen, All: all}
	commits, err := gitlog.Load(gitlog.Options{Dir: dir, Max: maxSeen, All: all})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ggraph:", err)
		os.Exit(1)
	}
	if len(commits) == 0 {
		fmt.Fprintln(os.Stderr, "ggraph: no commits found")
		os.Exit(1)
	}
	if dump {
		ui.Dump(os.Stdout, commits, !noColor)
		return
	}

	p := tea.NewProgram(ui.New(commits, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ggraph:", err)
		os.Exit(1)
	}
}
