// Package selection presents the generated commit message for user review.
package selection

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Choice is the action selected from the review menu.
type Choice int

const (
	// Accept keeps the generated commit message.
	Accept Choice = iota
	// Edit opens the generated message for editing.
	Edit
	// Regenerate requests another commit message.
	Regenerate
)

// Result contains the action selected from the review menu.
type Result struct {
	// Choice is the action selected by the user.
	Choice Choice
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
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
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

// Run displays commit and returns the user's menu choice.
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
