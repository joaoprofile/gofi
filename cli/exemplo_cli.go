//go:build ignore

// Reference for the terminal look of the interactive dialogues (tui/flow).
// Kept out of the build; run it alone with `go run exemplo_cli.go`.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- Cores e estilos ----------

var (
	accent  = lipgloss.Color("#4FB6A8") // cor do mascote / destaques
	dim     = lipgloss.Color("#6B6B6B")
	border  = lipgloss.Color("#4A4A5A")
	green   = lipgloss.Color("#4CC38A")
	red     = lipgloss.Color("#E5484D")
	white   = lipgloss.Color("#EDEDED")
	userBg  = lipgloss.Color("#353535")
	pathCol = lipgloss.Color("#8B8BF5")

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(white)
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	boldStyle   = lipgloss.NewStyle().Bold(true).Foreground(white)
	borderStyle = lipgloss.NewStyle().Foreground(border)
)

// Mascote original em pixel art (blocos Unicode)
const mascot = `  ▄████▄
 ██▀██▀██
 ████████
 ██▀▄▀▄██`

// ---------- Mensagens assíncronas ----------

type cmdResult struct {
	command string
	output  string
	err     error
}

func runShell(command string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("bash", "-c", command).CombinedOutput()
		return cmdResult{command: command, output: string(out), err: err}
	}
}

// ---------- Model ----------

type model struct {
	input   textinput.Model
	spin    spinner.Model
	width   int
	running bool
}

func newModel() model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(white)
	ti.Placeholder = "Digite uma mensagem ou !comando"
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = accentStyle

	return model{input: ti, spin: sp, width: 80}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.Println(header()))
}

// ---------- Renderização dos blocos ----------

func header() string {
	cwd, _ := os.Getwd()
	info := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Mini Terminal ")+dimStyle.Render("v0.1.0"),
		dimStyle.Render("Go + Bubble Tea · Demo"),
		dimStyle.Render(cwd),
	)
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Center,
		accentStyle.Render(mascot), "   ", info) + "\n"
}

func (m model) userLine(text string) string {
	return lipgloss.NewStyle().
		Background(userBg).
		Foreground(white).
		Width(m.width).
		Render("> " + text)
}

func (m model) bullet(color lipgloss.Color, body string) string {
	b := lipgloss.NewStyle().Foreground(color).Render("● ")
	wrapped := lipgloss.NewStyle().Width(m.width - 2).Render(body)
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, b, wrapped)
}

func (m model) assistant(text string) string {
	return m.bullet(white, text)
}

func (m model) toolCall(r cmdResult) string {
	color := green
	if r.err != nil {
		color = red
	}
	title := boldStyle.Render("Bash") + "(" + r.command + ")"

	out := strings.TrimRight(r.output, "\n")
	if out == "" {
		out = "(sem saída)"
	}
	lines := strings.Split(out, "\n")
	const maxLines = 8
	extra := 0
	if len(lines) > maxLines {
		extra = len(lines) - maxLines
		lines = lines[:maxLines]
	}

	var sb strings.Builder
	for i, l := range lines {
		prefix := "     "
		if i == 0 {
			prefix = "  ⎿  "
		}
		sb.WriteString("\n" + dimStyle.Render(prefix) + l)
	}
	if extra > 0 {
		sb.WriteString("\n" + dimStyle.Render(fmt.Sprintf("     … +%d linhas", extra)))
	}
	return m.bullet(color, title) + sb.String()
}

// ---------- Update ----------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.running {
				return m, nil
			}
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.input.Reset()

			if val == "/exit" {
				return m, tea.Quit
			}

			cmds := []tea.Cmd{tea.Println("\n" + m.userLine(val))}

			if strings.HasPrefix(val, "!") {
				command := strings.TrimSpace(val[1:])
				m.running = true
				cmds = append(cmds,
					tea.Println(m.assistant("Vou executar este comando: "+
						lipgloss.NewStyle().Foreground(pathCol).Render(command))),
					runShell(command),
					m.spin.Tick,
				)
			} else {
				reply := fmt.Sprintf("Você disse %q. Aqui você plugaria uma chamada a um LLM. "+
					"Use !comando para rodar algo no shell.", val)
				cmds = append(cmds, tea.Println(m.assistant(reply)))
			}
			return m, tea.Batch(cmds...)
		}

	case cmdResult:
		m.running = false
		summary := "Comando executado com sucesso."
		if msg.err != nil {
			summary = "O comando falhou: " + msg.err.Error()
		}
		return m, tea.Sequence(
			tea.Println(m.toolCall(msg)),
			tea.Println(m.assistant(summary)),
		)

	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ---------- View (só a parte "viva": input + rodapé) ----------

func (m model) View() string {
	line := borderStyle.Render(strings.Repeat("─", m.width))

	status := dimStyle.Render("  ? for shortcuts · !cmd roda no shell · /exit sai")
	if m.running {
		status = "  " + m.spin.View() + accentStyle.Render(" Executando…")
	}

	return "\n" + line + "\n" + m.input.View() + "\n" + line + "\n" + status
}

func main() {
	if _, err := tea.NewProgram(newModel()).Run(); err != nil {
		fmt.Println("erro:", err)
		os.Exit(1)
	}
}