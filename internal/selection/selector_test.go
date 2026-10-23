package selection

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunReturnsSelectedChoiceWithoutTTY(t *testing.T) {
	withProgramRunner(t, func(initial tea.Model) (tea.Model, error) {
		if !strings.Contains(initial.View(), "feat: test selection") {
			t.Fatalf("initial view = %q, want generated message", initial.View())
		}
		updated, _ := initial.Update(keyMessage("enter"))
		return updated, nil
	})

	result, err := Run("feat: test selection")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Choice != Accept {
		t.Fatalf("Run() choice = %d, want %d", result.Choice, Accept)
	}
}

func TestRunReturnsProgramError(t *testing.T) {
	wantErr := errors.New("program failed")
	withProgramRunner(t, func(tea.Model) (tea.Model, error) {
		return nil, wantErr
	})

	if _, err := Run("feat: test selection"); !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRunRejectsExitWithoutSelection(t *testing.T) {
	withProgramRunner(t, func(initial tea.Model) (tea.Model, error) {
		updated, _ := initial.Update(keyMessage("q"))
		return updated, nil
	})

	if _, err := Run("feat: test selection"); err == nil || err.Error() != "no selection made" {
		t.Fatalf("Run() error = %v, want no selection error", err)
	}
}

func withProgramRunner(t *testing.T, runner func(tea.Model) (tea.Model, error)) {
	t.Helper()
	original := runProgram
	runProgram = runner
	t.Cleanup(func() { runProgram = original })
}

func TestModelInitAndView(t *testing.T) {
	m := model{
		commit:  "fix: preserve selected provider key",
		choices: []string{"Accept", "Edit", "Regenerate"},
		cursor:  1,
	}
	if m.Init() != nil {
		t.Fatal("Init() command is not nil")
	}

	want := "fix: preserve selected provider key\n\n Accept\n> Edit\n Regenerate\n\n(↑/↓ to move, enter to select)"
	if got := m.View(); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelUpdate(t *testing.T) {
	tests := []modelUpdateCase{
		{name: "up at first choice", msg: keyMessage("up"), cursor: 0, wantCursor: 0},
		{name: "k moves up", msg: keyMessage("k"), cursor: 1, wantCursor: 0},
		{name: "down moves down", msg: keyMessage("down"), cursor: 1, wantCursor: 2},
		{name: "j moves down", msg: keyMessage("j"), cursor: 0, wantCursor: 1},
		{name: "down at last choice", msg: keyMessage("down"), cursor: 2, wantCursor: 3},
		{name: "enter selects cursor", msg: keyMessage("enter"), cursor: 2, wantCursor: 2, wantChoice: choicePointer(Regenerate), wantQuit: true},
		{name: "q quits", msg: keyMessage("q"), cursor: 1, wantCursor: 1, wantQuit: true},
		{name: "ctrl c quits", msg: keyMessage("ctrl+c"), cursor: 1, wantCursor: 1, wantQuit: true},
		{name: "unknown key", msg: keyMessage("x"), cursor: 1, wantCursor: 1},
		{name: "non-key message", msg: tea.WindowSizeMsg{Width: 80, Height: 24}, cursor: 1, wantCursor: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := model{
				commit:  "fix: test commit",
				choices: []string{"Accept", "Edit", "Regenerate"},
				cursor:  test.cursor,
			}
			updated, cmd := m.Update(test.msg)
			assertModelUpdate(t, test, updated, cmd)
		})
	}
}

type modelUpdateCase struct {
	name       string
	cursor     int
	msg        tea.Msg
	wantCursor int
	wantChoice *Choice
	wantQuit   bool
}

func assertModelUpdate(t *testing.T, test modelUpdateCase, updated tea.Model, cmd tea.Cmd) {
	t.Helper()
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() model type = %T, want model", updated)
	}
	if got.cursor != test.wantCursor {
		t.Fatalf("Update() cursor = %d, want %d", got.cursor, test.wantCursor)
	}
	assertSelectedChoice(t, got.result, test.wantChoice)
	assertQuitCommand(t, cmd, test.wantQuit)
}

func assertSelectedChoice(t *testing.T, got *Result, want *Choice) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("Update() result = %#v, want nil", got)
		}
		return
	}
	if got == nil {
		t.Fatal("Update() result = nil, want selected choice")
	}
	if got.Choice != *want {
		t.Fatalf("Update() result choice = %d, want %d", got.Choice, *want)
	}
}

func assertQuitCommand(t *testing.T, cmd tea.Cmd, wantQuit bool) {
	t.Helper()
	if !wantQuit {
		if cmd != nil {
			t.Fatal("Update() command is not nil")
		}
		return
	}
	if cmd == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Update() command message is not QuitMsg")
	}
}

func keyMessage(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func choicePointer(choice Choice) *Choice {
	return &choice
}
