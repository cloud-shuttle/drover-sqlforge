package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/drover-org/drover-sqlforge/internal/plan"
)

var (
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	subTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1)
)

type modelState struct {
	name      string
	status    plan.EventType
	err       error
	startTime time.Time
	duration  time.Duration
}

type tuiModel struct {
	models    []*modelState
	modelIdx  map[string]int
	spinner   spinner.Model
	eventChan <-chan plan.ApplyEvent
	done      bool
	quitting  bool
	err       error
	total     int
	completed int
}

type updateMsg plan.ApplyEvent
type doneMsg struct{}

func waitForEvent(c <-chan plan.ApplyEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-c
		if !ok {
			return doneMsg{}
		}
		return updateMsg(ev)
	}
}

func initialModel(eventChan <-chan plan.ApplyEvent, total int) tuiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return tuiModel{
		models:    []*modelState{},
		modelIdx:  make(map[string]int),
		spinner:   s,
		eventChan: eventChan,
		total:     total,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForEvent(m.eventChan))
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
	case updateMsg:
		idx, exists := m.modelIdx[msg.ModelName]
		if !exists {
			idx = len(m.models)
			m.modelIdx[msg.ModelName] = idx
			m.models = append(m.models, &modelState{
				name:   msg.ModelName,
				status: plan.EventStart,
			})
		}

		state := m.models[idx]
		state.status = msg.Type

		if msg.Type == plan.EventStart {
			state.startTime = time.Now()
		} else if msg.Type == plan.EventSuccess || msg.Type == plan.EventError {
			state.duration = time.Since(state.startTime)
			state.err = msg.Error
			m.completed++
			if msg.Type == plan.EventError {
				m.err = msg.Error
			}
		}

		return m, waitForEvent(m.eventChan)

	case doneMsg:
		m.done = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m tuiModel) View() string {
	if m.quitting {
		return "Aborted.\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("SQLForge Apply Pipeline"))
	b.WriteString("\n")
	b.WriteString(subTitleStyle.Render(fmt.Sprintf("Applying %d models to environment...", m.total)))
	b.WriteString("\n")

	for _, s := range m.models {
		var icon string
		var timing string

		if s.status == plan.EventStart {
			icon = m.spinner.View()
			timing = pendingStyle.Render("executing...")
		} else if s.status == plan.EventSuccess {
			icon = successStyle.Render("✓")
			timing = successStyle.Render(fmt.Sprintf("done in %v", s.duration.Round(time.Millisecond)))
		} else if s.status == plan.EventError {
			icon = errorStyle.Render("✗")
			timing = errorStyle.Render(fmt.Sprintf("failed: %v", s.err))
		}

		b.WriteString(fmt.Sprintf(" %s %s (%s)\n", icon, s.name, timing))
	}

	if m.done {
		b.WriteString("\n")
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Pipeline failed: %v", m.err)))
		} else {
			b.WriteString(successStyle.Render(fmt.Sprintf("✨ Successfully applied %d models!", m.total)))
		}
		b.WriteString("\n")
	}

	return b.String()
}
