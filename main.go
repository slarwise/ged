package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var FILENAME string

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Must provide a filename as the first argument")
		os.Exit(1)
	}
	FILENAME = os.Args[1]
	fileToEdit, err := os.Open(FILENAME)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open file %s: %s\n", FILENAME, err.Error())
		os.Exit(1)
	}
	defer fileToEdit.Close()
	contents, err := io.ReadAll(fileToEdit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read file %s: %s\n", FILENAME, err.Error())
		os.Exit(1)
	}

	var logger *slog.Logger
	if os.Getenv("GED_LOG") != "" {
		logFile, err := os.Create("log")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open log file: %s\n", err.Error())
			os.Exit(1)
		}
		defer logFile.Close()
		logger = slog.New(slog.NewJSONHandler(logFile, nil))
	} else {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	slog.SetDefault(logger)

	p := tea.NewProgram(model{mode: NORMAL_MODE, text: string(contents)})
	if _, err := p.Run(); err != nil {
		slog.Error("Exited with error", "err", err.Error())
	}
}

type model struct {
	text   string
	mode   string
	status string
}

const (
	NORMAL_MODE  = "normal"
	INSERT_MODE  = "insert"
	COMMAND_MODE = "command"
)

type errMsg int

const (
	COULD_NOT_WRITE_FILE = iota
)

type successMsg int

const (
	FILE_WRITTEN = iota
)

func (m model) Init() tea.Cmd {
	return tea.ClearScreen
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	slog.Info("Update", "mode", m.mode, "msg", msg)
	m.status = ""
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case errMsg:
		switch msg {
		case COULD_NOT_WRITE_FILE:
			m.status = "Could not write file"
		}
	case successMsg:
		switch msg {
		case FILE_WRITTEN:
			m.status = fmt.Sprintf("%s written", FILENAME)
		}
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			cmds = append(cmds, tea.Quit)
		}
	}
	switch m.mode {
	case NORMAL_MODE:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeySpace:
				m.mode = COMMAND_MODE
			case tea.KeyCtrlL:
				m.status = ""
			case tea.KeyRunes:
				switch string(msg.Runes) {
				case "i":
					m.mode = INSERT_MODE
				case "q":
					cmds = append(cmds, tea.Quit)
				}
			}
		}
	case INSERT_MODE:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEscape:
				m.mode = NORMAL_MODE
			case tea.KeyBackspace:
				m.text = m.text[0:max(0, len(m.text)-1)]
			case tea.KeyEnter:
				m.text += "\n"
			case tea.KeySpace:
				m.text += " "
			case tea.KeyRunes:
				m.text += string(msg.Runes[0])
			}
		}
	case COMMAND_MODE:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEscape:
				m.mode = NORMAL_MODE
			case tea.KeyRunes:
				switch string(msg.Runes) {
				case "w":
					m.mode = NORMAL_MODE
					cmds = append(cmds, writeFile(m.text))
				case "x":
					cmds = append(cmds, writeFile(m.text), tea.Quit)
				}
			}
		}
	}
	if len(cmds) < 2 {
		return m, tea.Batch(cmds...)
	} else {
		return m, tea.Sequence(cmds...)
	}
}

var (
	TEXT_STYLE = lipgloss.NewStyle().
			PaddingLeft(2).
			Width(80)
	HELP_STYLE = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2)
	HELP_NORMAL = HELP_STYLE.
			Copy().
			Background(lipgloss.Color("6")).
			Foreground(lipgloss.Color("0")).
			Render("NORMAL <i: insert mode> <space: command mode> <q: quit>")
	HELP_INSERT = HELP_STYLE.
			Copy().
			Background(lipgloss.Color("8")).
			Foreground(lipgloss.Color("7")).
			Render("INSERT <esc: normal mode>")
	HELP_COMMAND = HELP_STYLE.
			Copy().
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("0")).
			Render("COMMAND <w: write> <x: write and quit> <esc: normal mode>")
	STATUS_STYLE = lipgloss.NewStyle().
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("0")).
			PaddingLeft(2).
			PaddingRight(2).
			Italic(true)
)

func (m model) View() string {
	var help string
	switch m.mode {
	case NORMAL_MODE:
		help = HELP_NORMAL
	case INSERT_MODE:
		help = HELP_INSERT
	case COMMAND_MODE:
		help = HELP_COMMAND
	}
	status := STATUS_STYLE.Render(m.status)
	text := TEXT_STYLE.Render(m.text)
	if m.status != "" {
		return fmt.Sprintf("%s\n%s\n", text, status)
	} else {
		return fmt.Sprintf("%s\n%s\n", text, help)
	}
}

func writeFile(text string) tea.Cmd {
	return func() tea.Msg {
		if err := os.WriteFile(FILENAME, []byte(text), 0644); err != nil {
			return errMsg(COULD_NOT_WRITE_FILE)
		}
		return successMsg(FILE_WRITTEN)
	}
}
