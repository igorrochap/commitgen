package selection

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Choice int

const (
	Accept Choice = iota
	Edit
	Regenerate
)

type Result struct {
	Choice       Choice
	EditedCommit string
}

type model struct {
	commit  string
	choices []string
	cursor  int
	result  *Result
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor++
			}
		case "down", "j":
			if m.cursor < len(m.choices) {
				m.cursor++
			}
		case "enter":
			m.result = &Result{Choice: Choice(m.cursor)}
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("%s\n\n", m.commit)
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s%s\n", cursor, choice)
	}
	s += "\n(↑/↓ to move, enter to select)"
	return s
}

func Run(commit string) (Result, error) {
	m := model{
		commit:  commit,
		choices: []string{"Accept", "Edit", "Regenerate"},
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	finalModel := final.(model)
	if finalModel.result == nil {
		return Result{}, fmt.Errorf("no selection made")
	}
	return *finalModel.result, nil
}
