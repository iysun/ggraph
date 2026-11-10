// Package ui renders and drives the interactive git graph.
package ui

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iysun/ggraph/internal/gitlog"
)

type mode int

const (
	modeList mode = iota
	modeDetail
	modeHelp
)

// Config carries the options needed to reload the log.
type Config struct {
	Dir string
	Max int
	All bool
}

type line struct {
	commit int // index into commits, -1 for graph-only rows
	graph  string
}

// Model is the bubbletea model.
type Model struct {
	cfg     Config
	commits []gitlog.Commit
	lines   []line
	sel     []int // line index of each commit
	cursor  int   // index into sel
	offset  int   // first visible line
	width   int
	height  int

	mode   mode
	detail []string
	dOff   int
	patch  bool
	status string
}

// New builds a model for commits already loaded from cfg.
func New(commits []gitlog.Commit, cfg Config) *Model {
	m := &Model{cfg: cfg}
	m.setCommits(commits)
	return m
}

func (m *Model) setCommits(cs []gitlog.Commit) {
	m.commits = cs
	m.lines = nil
	m.sel = nil
	for i, c := range cs {
		m.sel = append(m.sel, len(m.lines))
		m.lines = append(m.lines, line{commit: i, graph: c.Graph})
		for _, t := range c.Tail {
			m.lines = append(m.lines, line{commit: -1, graph: t})
		}
	}
	if m.cursor >= len(m.sel) {
		m.cursor = len(m.sel) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

var (
	graphStyles = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
	styleHash    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHead    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleBranch  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleRemote  = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleTag     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleTitle   = lipgloss.NewStyle().Bold(true)
	styleCursor  = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})
	styleHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stylePager   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleSubject = lipgloss.NewStyle()
)

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()
	case detailMsg:
		if msg.err != "" {
			m.status = "git show: " + msg.err
			return m, nil
		}
		m.detail = strings.Split(strings.TrimRight(msg.text, "\n"), "\n")
		m.dOff = 0
		m.mode = modeDetail
	case reloadMsg:
		if msg.err != "" {
			m.status = msg.err
			return m, nil
		}
		m.setCommits(msg.commits)
		m.offset = 0
		m.status = fmt.Sprintf("reloaded %d commits", len(msg.commits))
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeDetail:
		switch k.String() {
		case "esc", "q":
			m.mode = modeList
		case "j", "down":
			m.scrollDetail(1)
		case "k", "up":
			m.scrollDetail(-1)
		case "g":
			m.dOff = 0
		case "G":
			m.dOff = max(0, len(m.detail)-m.bodyHeight())
		case "pgdown", "ctrl+f":
			m.scrollDetail(m.bodyHeight())
		case "pgup", "ctrl+b":
			m.scrollDetail(-m.bodyHeight())
		case "p":
			m.patch = !m.patch
			c := m.commits[m.sel[m.cursor]]
			return m, m.loadDetail(c.Hash, m.patch)
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	case modeHelp:
		switch k.String() {
		case "esc", "q", "?":
			m.mode = modeList
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		m.cursor = 0
		m.clampOffset()
	case "G":
		m.cursor = len(m.sel) - 1
		m.clampOffset()
	case "ctrl+d", "pgdown":
		m.move(m.bodyHeight())
	case "ctrl+u", "pgup":
		m.move(-m.bodyHeight())
	case "enter":
		c := m.commits[m.sel[m.cursor]]
		m.patch = false
		return m, m.loadDetail(c.Hash, false)
	case "r":
		return m, m.reload()
	}
	return m, nil
}

func (m *Model) move(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.sel)-1 {
		m.cursor = len(m.sel) - 1
	}
	m.clampOffset()
}

func (m *Model) clampOffset() {
	h := m.bodyHeight()
	if m.cursor < len(m.sel) && len(m.sel) > 0 {
		cur := m.sel[m.cursor]
		if cur < m.offset {
			m.offset = cur
		}
		if cur >= m.offset+h {
			m.offset = cur - h + 1
		}
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if maxOff := len(m.lines) - h; m.offset > maxOff {
		m.offset = max(0, maxOff)
	}
}

func (m *Model) scrollDetail(delta int) {
	m.dOff += delta
	if m.dOff < 0 {
		m.dOff = 0
	}
	if maxOff := len(m.detail) - m.bodyHeight(); m.dOff > maxOff {
		m.dOff = max(0, maxOff)
	}
}

func (m *Model) bodyHeight() int {
	h := m.height - 3
	if h < 1 {
		h = 1
	}
	return h
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.mode == modeDetail {
		return m.detailView()
	}
	if m.mode == modeHelp {
		return m.helpView()
	}

	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	limit := min(len(m.lines), m.offset+m.bodyHeight())
	for i := m.offset; i < limit; i++ {
		selected := m.cursor < len(m.sel) && m.sel[m.cursor] == i
		b.WriteString(m.lineView(m.lines[i], selected))
		b.WriteString("\n")
	}
	for i := limit; i < m.offset+m.bodyHeight(); i++ {
		b.WriteString("\n")
	}
	b.WriteString(m.footer())
	return b.String()
}

func (m *Model) header() string {
	name := m.cfg.Dir
	if abs, err := filepath.Abs(name); err == nil {
		name = filepath.Base(abs)
	}
	left := styleTitle.Render("ggraph") + styleDim.Render(" · "+name)
	right := styleDim.Render(fmt.Sprintf("%d commits · %d/%d", len(m.commits), m.cursor+1, len(m.sel)))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) footer() string {
	if m.status != "" {
		return stylePager.Render(truncate(m.status, m.width))
	}
	return styleHint.Render(truncate("j/k move · enter detail · r reload · ? help · q quit", m.width))
}

func (m *Model) detailView() string {
	c := m.commits[m.sel[m.cursor]]
	title := styleTitle.Render(c.Short+" ") + styleDim.Render(c.Author+" · "+c.When)
	if m.patch {
		title += styleDim.Render(" · patch")
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	limit := min(len(m.detail), m.dOff+m.bodyHeight())
	for i := m.dOff; i < limit; i++ {
		b.WriteString(truncate(m.detail[i], m.width) + "\n")
	}
	for i := limit; i < m.dOff+m.bodyHeight(); i++ {
		b.WriteString("\n")
	}
	b.WriteString(styleHint.Render(truncate("j/k scroll · p toggle patch · esc back · q quit", m.width)))
	return b.String()
}

func (m *Model) helpView() string {
	help := []string{
		"",
		"  ggraph — interactive git graph",
		"",
		"  j / ↓        next commit          k / ↑        previous commit",
		"  ctrl+d / pgdn  page down          ctrl+u / pgup  page up",
		"  g            first commit         G            last commit",
		"  enter        commit detail        p (detail)   toggle full patch",
		"  r            reload               ?            this help",
		"  q            quit",
		"",
		"  The lane art comes from `git log --graph`; refs, colors and the",
		"  detail pane are rendered by ggraph.",
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render(" help ") + "\n")
	for _, l := range help {
		b.WriteString(truncate(l, m.width) + "\n")
	}
	b.WriteString(styleHint.Render("esc/q back"))
	return b.String()
}

func (m *Model) lineView(ln line, selected bool) string {
	gut := "  "
	if selected {
		gut = "> "
	}
	text := gut + colorGraph(strings.TrimRight(ln.graph, " "))
	if ln.commit >= 0 {
		c := m.commits[ln.commit]
		text += " " + styleHash.Render(c.Short)
		if refs := refsView(c.Refs); refs != "" {
			text += " " + refs
		}
		text += " " + styleSubject.Render(c.Subject)
	}
	text = truncate(text, m.width)
	if selected {
		text = styleCursor.Render(padRight(text, m.width))
	}
	return text
}

// colorGraph colors each lane column, two characters per lane.
func colorGraph(g string) string {
	var b strings.Builder
	for i, r := range g {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		b.WriteString(graphStyles[(i/2)%len(graphStyles)].Render(string(r)))
	}
	return b.String()
}

func refsView(raw string) string {
	if raw == "" {
		return ""
	}
	var parts []string
	for _, ref := range strings.Split(raw, ", ") {
		switch {
		case strings.HasPrefix(ref, "HEAD"):
			parts = append(parts, styleHead.Render(ref))
		case strings.HasPrefix(ref, "tag:"):
			parts = append(parts, styleTag.Render(strings.TrimPrefix(ref, "tag: ")))
		case strings.Contains(ref, "/"):
			parts = append(parts, styleRemote.Render(ref))
		default:
			parts = append(parts, styleBranch.Render(ref))
		}
	}
	return strings.Join(parts, styleDim.Render(", "))
}

func padRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// truncate cuts s to at most width display cells, preserving ANSI sequences.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// ---- async commands ----

type detailMsg struct {
	text string
	err  string
}

type reloadMsg struct {
	commits []gitlog.Commit
	err     string
}

func (m *Model) loadDetail(hash string, patch bool) tea.Cmd {
	dir := m.cfg.Dir
	return func() tea.Msg {
		args := []string{"-c", "core.quotepath=false", "-C", dir, "show", "--no-color", "--format=medium", "--stat", hash}
		if patch {
			args = []string{"-c", "core.quotepath=false", "-C", dir, "show", "--no-color", "--format=medium", hash}
		}
		var out, errb bytes.Buffer
		cmd := exec.Command("git", args...)
		cmd.Stdout = &out
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			return detailMsg{err: strings.TrimSpace(errb.String())}
		}
		return detailMsg{text: out.String()}
	}
}

func (m *Model) reload() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		cs, err := gitlog.Load(gitlog.Options{Dir: cfg.Dir, Max: cfg.Max, All: cfg.All})
		if err != nil {
			return reloadMsg{err: err.Error()}
		}
		return reloadMsg{commits: cs}
	}
}

// Dump writes the graph without the TUI, for scripts and tests.
func Dump(w io.Writer, commits []gitlog.Commit, color bool) {
	for _, c := range commits {
		g := strings.TrimRight(c.Graph, " ")
		if color {
			g = colorGraph(g)
		}
		fmt.Fprintf(w, "%s %s%s\n", g, c.Short, subjectWithRefs(c, color))
		for _, t := range c.Tail {
			if color {
				t = colorGraph(strings.TrimRight(t, " "))
			}
			fmt.Fprintln(w, strings.TrimRight(t, " "))
		}
	}
}

func subjectWithRefs(c gitlog.Commit, color bool) string {
	var b strings.Builder
	if c.Refs != "" {
		refs := c.Refs
		if color {
			refs = refsView(refs)
		}
		b.WriteString(" " + refs)
	}
	b.WriteString(" " + c.Subject)
	return b.String()
}
