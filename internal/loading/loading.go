// Package loading displays progress while commitgen generates a commit message.
package loading

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	spinner            spinner.Model
	done               <-chan struct{}
	retryNotices       <-chan string
	noticeWriter       io.Writer
	completed          bool
	retryNoticesClosed bool
}

type doneMsg struct{}
type retryNoticeMsg string
type retryNoticesClosedMsg struct{}

func waitForDone(done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-done
		return doneMsg{}
	}
}

func (m model) Init() tea.Cmd {
	commands := []tea.Cmd{m.spinner.Tick, waitForDone(m.done)}
	if retryCommand := waitForRetryNotice(m.retryNotices); retryCommand != nil {
		commands = append(commands, retryCommand)
	}
	return tea.Batch(commands...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case doneMsg:
		m.completed = true
		if m.retryNotices == nil || m.retryNoticesClosed {
			return m, tea.Quit
		}
		return m, nil
	case retryNoticeMsg:
		if m.noticeWriter != nil {
			writeRetryNotice(m.noticeWriter, string(message))
		}
		return m, waitForRetryNotice(m.retryNotices)
	case retryNoticesClosedMsg:
		m.retryNoticesClosed = true
		if m.completed {
			return m, tea.Quit
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func writeRetryNotice(writer io.Writer, notice string) {
	if _, err := fmt.Fprintf(writer, "\n%s\n", notice); err != nil {
		log.Printf("loading retry notice: %v", err)
	}
}

func (m model) View() string {
	return fmt.Sprintf("Generating %s", m.spinner.View())
}

// Start begins the loading spinner and returns a function that waits for it to exit.
func Start(done <-chan struct{}) func() {
	return StartWithNotices(done, nil)
}

// StartWithNotices runs the loading spinner and keeps retry notices above it.
func StartWithNotices(done <-chan struct{}, retryNotices <-chan string) func() {
	finished := make(chan struct{})

	go func() {
		defer close(finished)

		s := spinner.New()
		s.Spinner = spinner.Points

		p := tea.NewProgram(
			model{spinner: s, done: done, retryNotices: retryNotices, noticeWriter: os.Stderr},
			tea.WithInput(nil),
		)
		if _, err := p.Run(); err != nil {
			log.Printf("loading spinner: %v", err)
		}
	}()

	return func() {
		<-finished
	}
}

func waitForRetryNotice(retryNotices <-chan string) tea.Cmd {
	if retryNotices == nil {
		return nil
	}
	return func() tea.Msg {
		notice, ok := <-retryNotices
		if !ok {
			return retryNoticesClosedMsg{}
		}
		return retryNoticeMsg(notice)
	}
}
