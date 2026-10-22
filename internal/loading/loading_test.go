package loading

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestWaitForDone(t *testing.T) {
	done := make(chan struct{})
	close(done)

	if _, ok := waitForDone(done)().(doneMsg); !ok {
		t.Fatal("waitForDone() message is not doneMsg")
	}
}

func TestModelInit(t *testing.T) {
	m := model{spinner: spinner.New(), done: make(chan struct{})}
	if m.Init() == nil {
		t.Fatal("Init() command = nil, want batch command")
	}
}

func TestModelUpdate(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.Msg
		wantQuit bool
	}{
		{name: "done", msg: doneMsg{}, wantQuit: true},
		{name: "spinner message", msg: tea.WindowSizeMsg{Width: 80, Height: 24}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := model{spinner: spinner.New(), done: make(chan struct{})}
			updated, cmd := m.Update(test.msg)
			if _, ok := updated.(model); !ok {
				t.Fatalf("Update() model type = %T, want model", updated)
			}
			if test.wantQuit {
				if cmd == nil {
					t.Fatal("Update() command = nil, want quit command")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("Update() command message is not QuitMsg")
				}
			}
		})
	}
}

func TestModelView(t *testing.T) {
	m := model{spinner: spinner.New()}
	if got := m.View(); !strings.HasPrefix(got, "Generating ") {
		t.Fatalf("View() = %q, want Generating prefix", got)
	}
}

func TestStartWaitsForDone(t *testing.T) {
	done := make(chan struct{})
	wait := Start(done)
	finished := make(chan struct{})
	go func() {
		wait()
		close(finished)
	}()

	select {
	case <-finished:
		t.Fatal("Start() wait returned before done was closed")
	case <-time.After(20 * time.Millisecond):
	}

	close(done)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Start() wait did not return after done was closed")
	}
}
