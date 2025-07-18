package tui

import (
	"fmt"
	"loom/config"
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
)

type model struct {
	workspacePath string
	config        *config.Config
	input         string
	messages      []string
	width         int
	height        int
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
		case "enter":
			if strings.TrimSpace(m.input) != "" {
				m.messages = append(m.messages, fmt.Sprintf("> %s", m.input))
				m.messages = append(m.messages, "Echo: "+m.input+" (No chat logic yet)")
				m.input = ""
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			m.input += msg.String()
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

	// Workspace info
	info := infoStyle.Render(fmt.Sprintf(
		"Workspace: %s\nModel: %s\nShell: %t",
		m.workspacePath,
		m.config.Model,
		m.config.EnableShell,
	))

	// Input area
	input := inputStyle.Render(fmt.Sprintf("Input: %s", m.input))

	// Messages area
	messageText := strings.Join(m.messages, "\n")
	if messageText == "" {
		messageText = "Welcome to Loom!\nType something and press Enter to test the interface.\nPress Ctrl+C or 'q' to exit."
	}
	messages := messageStyle.Render(messageText)

	// Help
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Press Ctrl+C or 'q' to quit • Enter to send message")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		"",
		input,
		"",
		messages,
		"",
		help,
	)
}

// StartTUI initializes and starts the TUI interface
func StartTUI(workspacePath string, cfg *config.Config) error {
	m := model{
		workspacePath: workspacePath,
		config:        cfg,
		input:         "",
		messages:      []string{},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
