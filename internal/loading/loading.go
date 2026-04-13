package loading

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	spinner spinner.Model
	done    <-chan struct{}
}

type doneMsg struct{}

func waitForDone(done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-done
		return doneMsg{}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForDone(m.done))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case doneMsg:
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m model) View() string {
	return fmt.Sprintf("Generating %s", m.spinner.View())
}

// Start begins the loading spinner and returns a wait function that blocks
// until the spinner has fully exited and the terminal is clean.
func Start(done <-chan struct{}) func() {
	finished := make(chan struct{})

	go func() {
		defer close(finished)

		s := spinner.New()
		s.Spinner = spinner.Points

		p := tea.NewProgram(
			model{spinner: s, done: done},
			tea.WithInput(nil),
		)
		p.Run() //nolint:errcheck
	}()

	return func() {
		<-finished
	}
}
