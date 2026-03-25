package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/naif/claude-history/internal/data"
	"github.com/naif/claude-history/internal/search"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	selectedTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				Background(lipgloss.Color("236"))

	researchStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	dateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Underline(true)

	snippetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	matchStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("211"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("62")).
			Padding(0, 1).
			Width(80)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type model struct {
	input     textinput.Model
	viewport  viewport.Model
	searcher  search.Searcher
	results   []data.SearchResult
	cursor    int
	width     int
	height    int
	searching bool
	lastQuery string
}

type searchDoneMsg struct {
	results []data.SearchResult
	err     error
}

func Run(searcher search.Searcher) error {
	ti := textinput.New()
	ti.Placeholder = "Search conversations..."
	ti.Focus()
	ti.Width = 60

	vp := viewport.New(80, 20)

	m := model{
		input:    ti,
		viewport: vp,
		searcher: searcher,
		width:    80,
		height:   24,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if len(m.results) > 0 {
				openURL(fmt.Sprintf("https://claude.ai/chat/%s", m.results[m.cursor].Conversation.UUID))
			}
			return m, nil
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.viewport.SetContent(m.renderResults())
				ensureCursorVisible(&m.viewport, m.cursor)
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.viewport.SetContent(m.renderResults())
				ensureCursorVisible(&m.viewport, m.cursor)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 4
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerH
		m.input.Width = msg.Width - 4
		if len(m.results) > 0 {
			m.viewport.SetContent(m.renderResults())
		}

	case searchDoneMsg:
		m.searching = false
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.results = msg.results
			m.cursor = 0
			m.viewport.SetContent(m.renderResults())
			m.viewport.GotoTop()
		}
		return m, nil
	}

	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	if m.input.Value() != prevVal {
		query := m.input.Value()
		if query != m.lastQuery && len(query) >= 2 {
			m.lastQuery = query
			m.searching = true
			cmds = append(cmds, m.doSearch(query))
		} else if len(query) < 2 {
			m.results = nil
			m.viewport.SetContent("")
		}
	}

	return m, tea.Batch(cmds...)
}

const entryHeight = 4 // title + date/url + snippet + blank line

func ensureCursorVisible(vp *viewport.Model, cursor int) {
	top := vp.YOffset
	bottom := top + vp.Height

	cursorTop := cursor * entryHeight
	cursorBottom := cursorTop + entryHeight

	if cursorTop < top {
		vp.SetYOffset(cursorTop)
	} else if cursorBottom > bottom {
		vp.SetYOffset(cursorBottom - vp.Height)
	}
}

func (m model) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.searcher.Search(context.Background(), query, 50)
		return searchDoneMsg{results: results, err: err}
	}
}

func (m model) View() string {
	header := headerStyle.Width(m.width).Render("Claude History Search")

	status := ""
	if m.searching {
		status = " searching..."
	} else if len(m.results) > 0 {
		status = fmt.Sprintf(" %d results", len(m.results))
	}

	help := helpStyle.Render("↑/↓/ctrl+p/n navigate • enter open • esc quit" + status)

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		header,
		m.input.View(),
		help,
		m.viewport.View(),
	)
}

func (m model) renderResults() string {
	if len(m.results) == 0 {
		return "No results found."
	}

	var b strings.Builder
	for i, r := range m.results {
		selected := i == m.cursor
		b.WriteString(m.renderEntry(i, r, selected))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) renderEntry(idx int, r data.SearchResult, selected bool) string {
	var b strings.Builder

	indicator := "  "
	ts := titleStyle
	if selected {
		indicator = cursorStyle.Render("▸ ")
		ts = selectedTitleStyle
	}

	title := ts.Render(fmt.Sprintf("%d. %s", idx+1, r.Conversation.Name))
	if r.Conversation.IsResearch {
		title += " " + researchStyle.Render("Research")
	}
	b.WriteString(indicator + title + "\n")

	date := dateStyle.Render(r.Conversation.CreatedAt.Format(time.DateOnly))
	url := urlStyle.Render(fmt.Sprintf("https://claude.ai/chat/%s", r.Conversation.UUID))
	b.WriteString("  " + date + "  " + url + "\n")

	if r.Snippet != "" {
		highlighted := highlightMatches(r.Snippet, r.MatchedTokens, m.width-4)
		b.WriteString("  " + highlighted)
	}

	return b.String()
}

func highlightMatches(snippet string, tokens []string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 80
	}
	clean := strings.Join(strings.Fields(snippet), " ")
	if len(clean) > maxLen {
		clean = clean[:maxLen-3] + "..."
	}

	if len(tokens) == 0 {
		return snippetStyle.Render(clean)
	}

	lower := strings.ToLower(clean)
	var spans []span

	for _, tok := range tokens {
		tokLower := strings.ToLower(tok)
		idx := 0
		for {
			pos := strings.Index(lower[idx:], tokLower)
			if pos < 0 {
				break
			}
			abs := idx + pos
			spans = append(spans, span{abs, abs + len(tokLower)})
			idx = abs + len(tokLower)
		}
	}

	if len(spans) == 0 {
		return snippetStyle.Render(clean)
	}

	merged := mergeSpans(spans, len(clean))

	var result strings.Builder
	prev := 0
	for _, s := range merged {
		if s.start > prev {
			result.WriteString(snippetStyle.Render(clean[prev:s.start]))
		}
		result.WriteString(matchStyle.Render(clean[s.start:s.end]))
		prev = s.end
	}
	if prev < len(clean) {
		result.WriteString(snippetStyle.Render(clean[prev:]))
	}

	return result.String()
}

func mergeSpans(spans []span, textLen int) []span {
	if len(spans) == 0 {
		return nil
	}

	// sort by start
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].start < spans[j-1].start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}

	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}

	// clamp
	for i := range merged {
		if merged[i].end > textLen {
			merged[i].end = textLen
		}
	}
	return merged
}

type span struct {
	start, end int
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return
	}
	cmd.Start()
}
