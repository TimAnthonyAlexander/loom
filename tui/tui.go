package tui

import (
	"fmt"
	"loom/config"
	"loom/indexer"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#04B575")).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#F25D94")).
			Padding(1, 2).
			Height(10)

	fileTreeStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#A550DF")).
			Padding(1, 2).
			Height(15)
)

type viewMode int

const (
	viewChat viewMode = iota
	viewFileTree
)

type model struct {
	workspacePath  string
	config         *config.Config
	index          *indexer.Index
	input          string
	messages       []string
	width          int
	height         int
	currentView    viewMode
	fileTreeScroll int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Switch between views
			if m.currentView == viewChat {
				m.currentView = viewFileTree
			} else {
				m.currentView = viewChat
			}
		case "up":
			if m.currentView == viewFileTree && m.fileTreeScroll > 0 {
				m.fileTreeScroll--
			}
		case "down":
			if m.currentView == viewFileTree {
				files := m.index.GetFileList()
				maxScroll := len(files) - 10 // Show 10 files at a time
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.fileTreeScroll < maxScroll {
					m.fileTreeScroll++
				}
			}
		case "enter":
			if m.currentView == viewChat && strings.TrimSpace(m.input) != "" {
				m.messages = append(m.messages, fmt.Sprintf("> %s", m.input))

				// Handle special commands
				if m.input == "/files" {
					stats := m.index.GetStats()
					m.messages = append(m.messages, fmt.Sprintf("Indexed %d files", stats.TotalFiles))
				} else if m.input == "/stats" {
					m.messages = append(m.messages, m.getIndexStatsMessage())
				} else {
					m.messages = append(m.messages, "Echo: "+m.input+" (No chat logic yet)")
				}
				m.input = ""
			}
		case "backspace":
			if m.currentView == viewChat && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if m.currentView == viewChat {
				m.input += msg.String()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m model) View() string {
	// Title
	title := titleStyle.Render("Loom - AI Coding Assistant")

	// Index stats for info panel
	stats := m.index.GetStats()
	langSummary := m.getLanguageSummary(stats)

	// Workspace info
	info := infoStyle.Render(fmt.Sprintf(
		"Workspace: %s\nModel: %s\nShell: %t\nFiles: %d %s",
		m.workspacePath,
		m.config.Model,
		m.config.EnableShell,
		stats.TotalFiles,
		langSummary,
	))

	var mainContent string

	if m.currentView == viewChat {
		// Input area
		input := inputStyle.Render(fmt.Sprintf("Input: %s", m.input))

		// Messages area
		messageText := strings.Join(m.messages, "\n")
		if messageText == "" {
			messageText = "Welcome to Loom!\nType something and press Enter to test.\nTry: /files or /stats\nPress Tab to view file tree.\nPress Ctrl+C or 'q' to exit."
		}
		messages := messageStyle.Render(messageText)

		mainContent = lipgloss.JoinVertical(lipgloss.Left, input, "", messages)
	} else {
		// File tree view
		treeContent := m.renderFileTree()
		fileTree := fileTreeStyle.Render(treeContent)
		mainContent = fileTree
	}

	// Help
	var helpText string
	if m.currentView == viewChat {
		helpText = "Press Tab for file tree • Ctrl+C or 'q' to quit • Enter to send message"
	} else {
		helpText = "Press Tab for chat • ↑↓ to scroll • Ctrl+C or 'q' to quit"
	}

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(helpText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		"",
		mainContent,
		"",
		help,
	)
}

func (m model) getLanguageSummary(stats indexer.IndexStats) string {
	if len(stats.LanguagePercent) == 0 {
		return ""
	}

	// Get top 3 languages
	type langPair struct {
		name    string
		percent float64
	}

	var langs []langPair
	for name, percent := range stats.LanguagePercent {
		if percent > 0 {
			langs = append(langs, langPair{name, percent})
		}
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].percent > langs[j].percent
	})

	var summary []string
	for i, lang := range langs {
		if i >= 3 {
			break
		}
		summary = append(summary, fmt.Sprintf("%s %.0f%%", lang.name, lang.percent))
	}

	if len(summary) > 0 {
		return "(" + strings.Join(summary, ", ") + ")"
	}

	return ""
}

func (m model) getIndexStatsMessage() string {
	stats := m.index.GetStats()

	var lines []string
	lines = append(lines, fmt.Sprintf("📊 Index Statistics"))
	lines = append(lines, fmt.Sprintf("Total files: %d", stats.TotalFiles))
	lines = append(lines, fmt.Sprintf("Total size: %.2f MB", float64(stats.TotalSize)/1024/1024))
	lines = append(lines, fmt.Sprintf("Last updated: %s", m.index.LastUpdated.Format("15:04:05")))

	if len(stats.LanguageBreakdown) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Language breakdown:")

		// Sort languages by count
		type langPair struct {
			name  string
			count int
		}

		var langs []langPair
		for name, count := range stats.LanguageBreakdown {
			if count > 0 {
				langs = append(langs, langPair{name, count})
			}
		}

		sort.Slice(langs, func(i, j int) bool {
			return langs[i].count > langs[j].count
		})

		for _, lang := range langs {
			percent := stats.LanguagePercent[lang.name]
			lines = append(lines, fmt.Sprintf("  %s: %d files (%.1f%%)",
				lang.name, lang.count, percent))
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderFileTree() string {
	files := m.index.GetFileList()

	if len(files) == 0 {
		return "No files indexed yet."
	}

	sort.Strings(files)

	var lines []string
	lines = append(lines, "📁 Indexed Files:")
	lines = append(lines, "")

	// Show files with pagination
	start := m.fileTreeScroll
	end := start + 10
	if end > len(files) {
		end = len(files)
	}

	for i := start; i < end; i++ {
		file := files[i]
		meta := m.index.Files[file]

		// Format file size
		var sizeStr string
		if meta.Size < 1024 {
			sizeStr = fmt.Sprintf("%dB", meta.Size)
		} else if meta.Size < 1024*1024 {
			sizeStr = fmt.Sprintf("%.1fKB", float64(meta.Size)/1024)
		} else {
			sizeStr = fmt.Sprintf("%.1fMB", float64(meta.Size)/1024/1024)
		}

		lines = append(lines, fmt.Sprintf("  %s (%s, %s)",
			file, sizeStr, meta.Language))
	}

	if len(files) > 10 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Showing %d-%d of %d files",
			start+1, end, len(files)))
	}

	return strings.Join(lines, "\n")
}

// StartTUI initializes and starts the TUI interface
func StartTUI(workspacePath string, cfg *config.Config, idx *indexer.Index) error {
	m := model{
		workspacePath: workspacePath,
		config:        cfg,
		index:         idx,
		input:         "",
		messages:      []string{},
		currentView:   viewChat,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
