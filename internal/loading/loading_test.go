package loading

import (
	"bytes"
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

func TestModelUpdateRetryNoticeKeepsSpinnerRunning(t *testing.T) {
	const notice = "gemini: 503 Service Unavailable, retrying in 1.3s (attempt 2/4)"
	var stderr bytes.Buffer
	m := model{spinner: spinner.New(), retryNotices: make(chan string), noticeWriter: &stderr}
	updated, next := m.Update(retryNoticeMsg(notice))
	updatedModel := updated.(model)
	if next == nil {
		t.Fatal("Update() command = nil, want next retry-notice command")
	}
	if got, want := stderr.String(), "\n"+notice+"\n"; got != want {
		t.Fatalf("retry notice output = %q, want %q", got, want)
	}
	before := updatedModel.View()
	if !strings.HasPrefix(before, "Generating ") {
		t.Fatalf("View() = %q, want spinner line", before)
	}
	updated, _ = updatedModel.Update(updatedModel.spinner.Tick())
	if got := updated.(model).View(); got == before {
		t.Fatalf("spinner view after notice tick = %q, want an updated frame", got)
	}
}

func TestModelWaitsForDoneAndRetryNoticesToClose(t *testing.T) {
	for _, doneFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "done first", false: "notices first"}[doneFirst], func(t *testing.T) {
			retryNotices := make(chan string)
			m := model{spinner: spinner.New(), done: make(chan struct{}), retryNotices: retryNotices}
			if doneFirst {
				m = updateModel(t, m, doneMsg{}, false)
				close(retryNotices)
				closedMessage := waitForRetryNotice(retryNotices)()
				_ = updateModel(t, m, closedMessage, true)
				return
			}
			close(retryNotices)
			closedMessage := waitForRetryNotice(retryNotices)()
			m = updateModel(t, m, closedMessage, false)
			_ = updateModel(t, m, doneMsg{}, true)
		})
	}
}

func updateModel(t *testing.T, current model, message tea.Msg, wantQuit bool) model {
	t.Helper()
	updated, command := current.Update(message)
	updatedModel, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() model type = %T, want model", updated)
	}
	if wantQuit {
		if command == nil {
			t.Fatal("Update() command = nil, want quit command")
		}
		if _, ok := command().(tea.QuitMsg); !ok {
			t.Fatal("Update() command message is not QuitMsg")
		}
		return updatedModel
	}
	if command != nil {
		t.Fatalf("Update() command = %v, want nil", command)
	}
	return updatedModel
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
