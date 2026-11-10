// Package gitlog loads commits from git in an order and shape that is
// convenient for rendering a graph.
package gitlog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const sep = "\x1f"

// Commit is one entry of `git log --graph`.
type Commit struct {
	Hash    string
	Short   string
	Parents []string
	Author  string
	When    string
	Refs    string   // raw %D decorations, e.g. "HEAD -> main, origin/main, tag: v1"
	Subject string   // first line of the commit message
	Graph   string   // graph art drawn before the commit on its own line
	Tail    []string // graph-only continuation lines that follow this commit
}

// Options controls Load.
type Options struct {
	// Dir is passed to git as `-C`; empty means the current directory.
	Dir string
	// Max limits the number of commits; 0 means no limit.
	Max int
	// All includes commits from every ref, not just HEAD.
	All bool
}

// Load runs `git log --graph` and parses the result.
func Load(opt Options) ([]Commit, error) {
	dir := opt.Dir
	if dir == "" {
		dir = "."
	}
	args := []string{
		"-c", "core.quotepath=false",
		"-C", dir, "log", "--graph", "--date-order", "--decorate=short", "--no-color",
		"--pretty=format:" + sep + "%H" + sep + "%P" + sep + "%an" + sep + "%ar" + sep + "%D" + sep + "%s",
	}
	if opt.All {
		args = append(args, "--all")
	}
	if opt.Max > 0 {
		args = append(args, fmt.Sprintf("-n%d", opt.Max))
	}

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git log failed: %s", msg)
	}
	return Parse(stdout.String()), nil
}

// Parse turns `git log --graph` output into commits.
//
// Each commit line looks like `* \x1f<hash>\x1f<parents>...`; the graph art
// precedes the first field separator. Lines without a separator are pure
// graph art (merge/continuation rows) and are attached to the commit above.
func Parse(out string) []Commit {
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if i := strings.Index(line, sep); i >= 0 {
			fields := strings.Split(line[i+1:], sep)
			if len(fields) < 6 {
				continue
			}
			c := Commit{
				Hash:    fields[0],
				Author:  fields[2],
				When:    fields[3],
				Refs:    fields[4],
				Subject: fields[5],
				Graph:   line[:i],
			}
			if fields[1] != "" {
				c.Parents = strings.Fields(fields[1])
			}
			if len(c.Hash) > 7 {
				c.Short = c.Hash[:7]
			}
			commits = append(commits, c)
			continue
		}
		if len(commits) > 0 && isGraphOnly(line) {
			commits[len(commits)-1].Tail = append(commits[len(commits)-1].Tail, line)
		}
	}
	return commits
}

func isGraphOnly(s string) bool {
	for _, r := range s {
		switch r {
		case '|', '/', '\\', '*', '^', '~', '_', '-', '.', ' ':
		default:
			return false
		}
	}
	return true
}
