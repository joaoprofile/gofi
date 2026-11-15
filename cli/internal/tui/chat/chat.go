// Package chat is gofi's interactive mode: a conversation with the agents in
// the terminal, laid out like the rest of the CLI — the banner on top, every
// finished step printed to the scrollback as a "● … ⎿ …" block, and only the
// answer being written kept live at the bottom.
//
// The conversation runs on an engine.Session; this package only renders it.
// Text in free form goes through the intake first, when there is one: the
// plan is shown, its questions asked, and its phases conducted one turn each
// (0001, D9). /skills go to the engine as typed, so every skill the project
// installed works with no wiring here. A few commands are the chat's own —
// /help, /clear, /model, /resume, /cost, /brief, /send, /exit and !command for
// the shell — and a few read the project without a model: /watch, /runs,
// /find, /show, /impact, /path.
//
// The files of the package, by what they draw: input.go the prompt, menu.go
// the list under it while a /command or an @path is typed, approve.go the
// dialog before a change, screen.go how it all reaches the terminal — and the
// board of the agents at work, pinned over the conversation while a run goes.
package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/approval"
	"github.com/joaoprofile/gofi/cli/internal/clipboard"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/ledger"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/tui/banner"
	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
	"github.com/joaoprofile/gofi/cli/internal/tui/watch"
)

// Options configure a chat.
type Options struct {
	Root      string           // the project root; the engine runs here
	Version   string           // shown in the banner
	Model     string           // configured model, until the engine names its own
	Approvals *approval.Server // where the engine asks before a change; nil when it does not

	// NewSession opens a conversation — a new one, or the saved one with that
	// id. Called at start, and again on /clear and /resume.
	NewSession func(resumeID string) engine.Session
	// Sessions lists the conversations that can be resumed, newest first.
	Sessions func() []engine.Saved
	// Models are offered by /model, in order.
	Models []string
	// Intake plans a request in free text: answers are the person's to the
	// last round's questions, proceed goes on without them. Nil sends every
	// request as typed.
	Intake func(text string, answers map[string]string, proceed bool) (*intake.Result, error)
	// TierModel is the model a tier runs on, for a phase the router raised
	// above its role's contract. Nil keeps the skill's own model.
	TierModel func(host.Tier) string
	// Deliver hands what a plan builds to the loop: every sealed change of a
	// context through a flow, its events as they come and, once they end, the
	// error the delivery ended with. Nil leaves the plan to stop there.
	Deliver func(flow, context string) Delivery
	// Review reads a context's open changes before the person approves them
	// (gofi-spec-review): the lines to show, and how many findings block.
	// What it does meanwhile goes to progress, a line at a time, so a review
	// of minutes is seen working. Nil leaves the review stop to the person
	// alone.
	Review func(ctx context.Context, contextName string, progress func(line string)) (lines []string, blocking int, err error)
	// Gofi builds a command of gofi itself, to run in the project: what
	// /watch opens and what /find, /show, /impact and /path ask the index.
	// Nil leaves those commands out.
	Gofi func(args ...string) *exec.Cmd
	// Clipboard reads the picture the clipboard holds, for ctrl+v. Nil reads
	// the system's.
	Clipboard func(context.Context) (clipboard.Picture, error)
	// Inset is how many rows at the top of the screen the terminal draws
	// over — an editor's terminal keeps the command that is running pinned
	// there. The chat leaves them empty, so its panel is not under them.
	Inset int
}

// Run holds the conversation until the user leaves.
func Run(opts Options) error {
	m := newModel(opts, styles.Enabled())
	p := tea.NewProgram(m)
	if opts.Approvals != nil {
		opts.Approvals.OnRequest(func(r *approval.Request) { p.Send(approvalMsg{r}) })
	}
	final, err := p.Run()
	if fm, ok := final.(model); ok && fm.session != nil {
		if fm.plan != nil {
			fm.plan.closePhase()
		}
		_ = fm.session.Close()
	}
	return err
}

// Messages from the engine, the shell and the approval channel back into the
// update loop.
type (
	eventMsg  struct{ event engine.Event }
	turnEnded struct{}
	shellDone struct {
		command string
		output  string
		err     error
		keep    int // lines of the output shown; 0 is a shell command's few
	}
	// described is what the engine said it runs, before any turn.
	described struct{ meta engine.Meta }
	// pasted is the picture a paste brought, or why there is none; quiet
	// when nobody asked for a picture — an empty paste.
	pasted struct {
		pic   clipboard.Picture
		err   error
		quiet bool
	}
	// watchClosed is the person back from the watch screen.
	watchClosed struct{ err error }
	tick        struct{}
	approvalMsg struct{ req *approval.Request }
	// deliveryEvent is one event of the delivery the loop carries out;
	// deliveryDone is its end.
	deliveryEvent struct{ event loop.Event }
	deliveryDone  struct{}
)

type model struct {
	opts    Options
	look    look
	color   bool
	session engine.Session

	input   prompt
	history []string
	histPos int
	// menu lists what a /command or an @path being typed could be; menuOff
	// is the person closing it, until they type again.
	menu    *menu
	menuOff bool

	// images are the pictures pasted into the prompt, each standing in the
	// text as "[Image #n]"; outgoing the ones the turn being sent takes.
	images   []attached
	imageSeq int
	outgoing []engine.Image

	width, height int

	// What reaches the terminal, and how (screen.go).
	queue    []string    // lines on their way to the scrollback
	flushing bool        // a batch of them is with the terminal
	window   []string    // the newest lines, held under the panel while it is pinned
	head     int         // how many of them, from the top, are the banner
	pinned   bool        // the panel is pinned and the chat draws the whole screen
	greeted  bool        // the banner was shown, once the terminal said its size
	leaving  bool        // the chat is ending: what is held goes, then it quits
	board    watch.Board // the runs alive in the project
	act      activity    // what the turn in flight has done, for the panel
	// steered are the messages sent while the turn runs, until the model
	// takes them.
	steered []steer

	running bool
	events  <-chan engine.Event
	started time.Time
	frame   int
	live    string // answer text streamed so far, not yet printed
	think   bool   // the model is thinking rather than writing

	tools map[string]string // tool use id → name, to label its result
	// calls are the tool calls as they were shown, and lastCall the one
	// shown last: a result that is not its own says which call it answers.
	calls    map[string]string
	lastCall string
	modelN   string
	skills   []string
	// onDisk are the project's skills as read from .claude/skills, and
	// described what the engine said it runs: the menu shows both.
	onDisk    []skill
	described []engine.Command

	// What the conversation holds and has cost: the tokens the last answer
	// read, the window they fill, and what the session spent.
	ctxTokens int
	ctxWindow int
	ctxWarned int     // the last size the person was told about
	spent     float64 // the conversation and what it conducted, so far
	lastCost  float64 // the engine's running total at the chat's last turn
	// ledger is what the conversation read and what that cost; fixes are
	// the repairs the last /audit listed, for /audit <n>.
	ledger *ledger.Ledger
	fixes  []ledger.Fix

	armedExit time.Time // first ctrl+c on an idle prompt
	notice    string    // one-line message in the status slot

	asking []*approval.Request // tool calls waiting for a yes or no, oldest first
	ask    asked               // the dialog over the first of them
	always map[string]bool     // what was approved for the rest of the conversation, by scope

	chosenModel string         // picked with /model; outlives /clear and /resume
	listed      []engine.Saved // what the last /resume listed, for /resume <n>

	plan       *plan       // the request being conducted, if any
	wait       string      // what the plan waits for from the person
	planning   bool        // the intake is running
	conducting bool        // the running turn is a phase of the plan
	lastFailed bool        // the last turn ended in error or was cut
	lastDone   engine.Done // how the last turn ended: its time and cost
}

func newModel(opts Options, color bool) model {
	onDisk := readProjectSkills(opts.Root)
	names := make([]string, 0, len(onDisk))
	for _, s := range onDisk {
		names = append(names, s.name)
	}
	return model{
		opts:    opts,
		look:    newLook(color),
		color:   color,
		session: opts.NewSession(""),
		input:   newPrompt(i18n.T("chat.placeholder")),
		width:   80,
		tools:   map[string]string{},
		calls:   map[string]string{},
		always:  map[string]bool{},
		modelN:  opts.Model,
		skills:  names,
		onDisk:  onDisk,
		ledger:  ledger.New(opts.Root),
	}
}

// Init starts what the chat needs before the first message. The banner is
// not said here: it waits for the terminal to say its size, which decides
// where it is drawn.
func (m model) Init() tea.Cmd {
	return tea.Batch(m.introduce(), watchBoard(m.opts.Root, 0))
}

// banner is the card the chat opens with: who it is and where, and beside
// that what the person can do from the prompt and the conversations they can
// go back to.
func (m model) banner() string {
	b := banner.Banner{
		Title:    "chat",
		Version:  m.opts.Version,
		Subtitle: i18n.T("chat.subtitle"),
		Dir:      m.opts.Root,
	}
	var about []string
	if m.modelN != "" {
		about = append(about, m.modelN)
	}
	if n := len(m.onDisk); n > 0 {
		about = append(about, i18n.T("chat.banner.skills", n))
	}
	if len(about) > 0 {
		b.Extra = []string{strings.Join(about, " · ")}
	}
	b.Side = []banner.Section{{Title: i18n.T("chat.banner.keys"), Lines: []string{
		"/\t" + i18n.T("chat.banner.slash"),
		"@  !\t" + i18n.T("chat.banner.at"),
		"alt+enter\t" + i18n.T("chat.banner.newline"),
		"ctrl+v\t" + i18n.T("chat.banner.image"),
		"enter\t" + i18n.T("chat.banner.steer"),
	}}}
	var recent []string
	if m.opts.Sessions != nil {
		for i, s := range m.opts.Sessions() {
			if i == 3 {
				break
			}
			recent = append(recent, ago(time.Since(s.Updated))+"\t"+strings.Join(strings.Fields(s.Title), " "))
		}
	}
	if len(recent) > 0 {
		b.Side = append(b.Side, banner.Section{Title: i18n.T("chat.banner.recent"), Lines: recent})
	} else {
		b.Side = append(b.Side, banner.Section{Title: i18n.T("chat.banner.free"), Lines: []string{
			"/find /show\t" + i18n.T("chat.banner.index"),
			"/runs /watch\t" + i18n.T("chat.banner.runs"),
			"/cost\t" + i18n.T("chat.banner.cost"),
		}})
	}
	return banner.RenderWidth(b, m.color, m.width)
}

// introduce starts the engine while the person reads the banner and asks it
// what it runs: by the first "/" the menu lists every command it knows, each
// with what it does. The engine answers for itself — no model is called.
func (m model) introduce() tea.Cmd {
	d, ok := m.session.(engine.Describer)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		meta, ok := d.Describe(ctx)
		if !ok {
			return nil
		}
		return described{meta}
	}
}

// Update takes a message and then lets the screen catch up: whatever the
// message said goes to the terminal in the order it was said.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next.(model).pump(cmd)
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(msg.Width - 3)
		if !m.greeted {
			// The first size: now the banner knows where it goes.
			m.greeted = true
			m.settle()
			m.emit(m.banner())
			if m.pinned {
				m.head = len(m.window)
			}
		}
		// A new size is drawn over the old frame, never after a clear: a
		// terminal that keeps what is cleared would file every frame away
		// in its scrollback.
		return m, nil

	case sayMsg:
		m.emit(msg.text)
		return m, nil
	case flushed:
		m.flushing = false
		return m, nil
	case boardMsg:
		m.board = msg.board
		return m, watchBoard(m.opts.Root, boardEvery(m.board))
	case described:
		m.describe(msg.meta)
		m.refreshMenu()
		return m, nil
	case pasted:
		return m.onPasted(msg)
	case watchClosed:
		if msg.err != nil {
			return m, say(m.look.nested([]string{msg.err.Error()}, m.look.warn))
		}
		return m, nil

	case tea.KeyMsg:
		if len(m.asking) > 0 {
			return m.onApprovalKey(msg)
		}
		return m.onKey(msg)

	case deliveryEvent:
		return m.onDeliveryEvent(msg.event)
	case deliveryDone:
		return m.onDeliveryDone()

	case approvalMsg:
		if m.remembered(msg.req) {
			msg.req.Answer(approval.Decision{Allow: true})
			return m, say(m.look.nested([]string{i18n.T("chat.approve.auto", scopeOf(msg.req).label)}, m.look.dim))
		}
		m.asking = append(m.asking, msg.req)
		if len(m.asking) == 1 {
			m.openAsk()
		}
		return m, nil

	case eventMsg:
		return m.onEvent(msg.event)

	case intakeDone:
		return m.onIntake(msg)

	case reviewed:
		return m.onReviewed(msg)
	case reviewProgress:
		return m, tea.Sequence(say(m.look.nested([]string{msg.line}, m.look.dim)), readReview(msg.ch))

	case turnEnded:
		m.running, m.events, m.think = false, nil, false
		m.asking, m.ask = nil, asked{}
		if len(m.steered) > 0 {
			// The engine ended before taking them: back to the prompt, so
			// what was written is not lost.
			var texts []string
			for _, st := range m.steered {
				texts = append(texts, st.text)
			}
			m.steered = nil
			if m.input.Empty() {
				m.input.SetValue(strings.Join(texts, "\n"))
			}
			m.notice = i18n.T("chat.steer.undelivered")
		}
		flush := m.flushLive()
		if m.conducting && m.plan != nil {
			next, cmd := m.afterPhase()
			return next, tea.Sequence(flush, cmd)
		}
		return m, flush

	case shellDone:
		m.running = false
		return m, say(m.shellBlock(msg))

	case tick:
		if !m.running {
			return m, nil
		}
		m.frame++
		return m, tickCmd()
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Paste {
		return m.onPaste(string(msg.Runes))
	}
	key := msg.String()
	switch key {
	case "ctrl+c":
		if m.running {
			m.interrupt()
			return m, nil
		}
		if m.stopDelivery() {
			return m, nil
		}
		if !m.input.Empty() {
			m.clearInput()
			return m, nil
		}
		if time.Since(m.armedExit) < 2*time.Second {
			m.leaving = true
			return m, nil
		}
		m.armedExit = time.Now()
		m.notice = i18n.T("chat.exit_hint")
		return m, nil
	case "ctrl+d":
		if m.input.Empty() {
			m.leaving = !m.running
			return m, nil
		}
		msg = tea.KeyMsg{Type: tea.KeyDelete}
	case "esc":
		if m.menu != nil {
			m.menu, m.menuOff = nil, true
			return m, nil
		}
		if m.running {
			m.interrupt()
		}
		m.stopDelivery()
		return m, nil
	case "ctrl+v", "alt+v":
		return m.pasteImage(false)
	}

	if m.menu != nil {
		switch key {
		case "up", "ctrl+p":
			m.menu.move(-1)
			return m, nil
		case "down", "ctrl+n":
			m.menu.move(1)
			return m, nil
		case "tab":
			return m.takeMenu(false)
		case "enter":
			if !m.running {
				return m.takeMenu(true)
			}
			// While a turn runs the menu only completes: what runs then is
			// decided by whileRunning.
			next, cmd := m.takeMenu(false)
			nm := next.(model)
			if text := strings.TrimSpace(nm.input.Value()); answeredHere[text] {
				return nm.whileRunning(text)
			}
			return nm, cmd
		}
	}

	switch key {
	case "up":
		// Up moves through the text first; from its first row, through what
		// was sent before.
		if m.input.Key(msg) {
			return m, nil
		}
		if m.histPos > 0 {
			m.histPos--
			m.input.SetValue(m.history[m.histPos])
		}
		return m, nil
	case "down":
		if m.input.Key(msg) {
			return m, nil
		}
		if m.histPos < len(m.history) {
			m.histPos++
			if m.histPos == len(m.history) {
				m.input.SetValue("")
			} else {
				m.input.SetValue(m.history[m.histPos])
			}
		}
		return m, nil
	case "tab":
		value, options := m.complete(m.input.Value())
		m.input.SetValue(value)
		m.notice = ""
		if len(options) > 0 {
			if len(options) > 8 {
				options = append(options[:8], "…")
			}
			m.notice = strings.Join(options, "  ")
		}
		return m, nil
	case "enter":
		// A backslash before enter breaks the line, as in a shell.
		if before := m.input.Before(); strings.HasSuffix(before, "\\") {
			at := len([]rune(before))
			m.input.Replace(at-1, at, "\n")
			return m, nil
		}
		if m.running {
			return m.whileRunning(strings.TrimSpace(m.input.Value()))
		}
		text := strings.TrimSpace(m.input.Value())
		if text == "" && (m.wait == waitConfirm || m.wait == waitContinue) {
			// Enter on an empty prompt takes the plan's default: go on.
			return m.onPlanReply("", nil)
		}
		return m.submit(text)
	}
	m.notice = ""
	if m.input.Key(msg) {
		m.menuOff = false
		m.refreshMenu()
	}
	return m, nil
}

// answeredHere are the commands that only read what the chat already has:
// they answer at once, a turn running or not.
var answeredHere = map[string]bool{"/help": true, "/cost": true, "/runs": true, "/audit": true}

// whileRunning takes what was typed while a turn runs. A message goes to
// the engine, which hands it to the model at its next step — the turn goes
// on knowing it —, and shows over the prompt until then. A command that only
// reads what the chat has is answered at once. Anything else waits for the
// turn: it stays in the prompt.
func (m model) whileRunning(text string) (tea.Model, tea.Cmd) {
	switch {
	case text == "":
		return m, nil
	case answeredHere[text]:
		return m.submit(text)
	case strings.HasPrefix(text, "/"), strings.HasPrefix(text, "!"), m.events == nil:
		m.notice = i18n.T("chat.steer.wait")
		return m, nil
	}
	st, ok := m.engine().(engine.Steerer)
	if !ok {
		m.notice = i18n.T("chat.steer.wait")
		return m, nil
	}
	m.takeImages(text)
	id, ok := st.Steer(engine.Turn{Prompt: text, Images: m.outgoing})
	m.outgoing = nil
	if !ok {
		// The turn ended as the key was pressed: enter again sends it.
		m.notice = i18n.T("chat.steer.ended")
		return m, nil
	}
	m.steered = append(m.steered, steer{id: id, text: text})
	m.input.Reset()
	m.menu, m.menuOff, m.notice = nil, false, ""
	if len(m.history) == 0 || m.history[len(m.history)-1] != text {
		m.history = append(m.history, text)
	}
	m.histPos = len(m.history)
	return m, nil
}

// clearInput empties the prompt and what was attached to it.
func (m *model) clearInput() {
	m.input.Reset()
	m.images, m.menu, m.menuOff = nil, nil, false
}

// refreshMenu opens, updates or closes the menu for what is under the cursor.
func (m *model) refreshMenu() {
	if m.menuOff {
		m.menu = nil
		return
	}
	next := m.openMenu()
	// The choice stays on the item it was on while it is still listed.
	if next != nil && m.menu != nil && m.menu.sel < len(m.menu.items) {
		was := m.menu.items[m.menu.sel].value
		for i, it := range next.items {
			if it.value == was {
				next.sel = i
				break
			}
		}
	}
	m.menu = next
}

// takeMenu puts the menu's choice in the prompt. With run, a command that
// needs nothing more is sent at once — enter on "/he" runs /help —; one that
// takes an argument, and any path, waits for the rest of the line.
func (m model) takeMenu(run bool) (tea.Model, tea.Cmd) {
	u := m.menu
	it := u.items[u.sel]
	value := it.value
	if !strings.HasSuffix(value, "/") {
		value += " "
	}
	m.input.Replace(u.from, len([]rune(m.input.Before())), value)
	m.menu = nil
	// Only the chat's own commands say "<this>" for what they cannot run
	// without; the engine writes its optional arguments the same way.
	needs := it.kind <= kindGofi && strings.HasPrefix(it.hint, "<")
	if u.path || !run || needs {
		m.refreshMenu()
		return m, nil
	}
	return m.submit(strings.TrimSpace(m.input.Value()))
}

// attached is a picture waiting to go with the next message.
type attached struct {
	n   int
	pic clipboard.Picture
}

func imageToken(n int) string { return fmt.Sprintf("[Image #%d]", n) }

// onPaste takes a paste: text goes into the prompt as it is, line breaks and
// all; the path of an image file brings the image; and an empty paste — what
// a terminal sends when the clipboard holds no text — asks the clipboard for
// a picture.
func (m model) onPaste(text string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(text) == "" {
		return m.pasteImage(true)
	}
	if path, ok := pastedPath(text); ok {
		return m, func() tea.Msg {
			pic, _, err := clipboard.File(path)
			return pasted{pic: pic, err: err}
		}
	}
	m.notice = ""
	m.input.Insert(text)
	m.menuOff = false
	m.refreshMenu()
	return m, nil
}

// pastedPath is the image file a pasted text names — what a terminal pastes
// when a file is dropped on it: one line, quoted or with its spaces escaped.
func pastedPath(text string) (string, bool) {
	p := strings.TrimSpace(text)
	if p == "" || strings.Contains(p, "\n") {
		return "", false
	}
	p = strings.TrimPrefix(strings.Trim(p, `'"`), "file://")
	p = strings.ReplaceAll(p, `\ `, " ")
	if !clipboard.IsImagePath(p) {
		return "", false
	}
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", false
	}
	return p, true
}

// pasteImage reads the clipboard's picture off the update loop: asking the
// system can take a second.
func (m model) pasteImage(quiet bool) (tea.Model, tea.Cmd) {
	read := m.opts.Clipboard
	if read == nil {
		read = clipboard.Read
	}
	if !quiet {
		m.notice = i18n.T("chat.image.reading")
	}
	return m, func() tea.Msg {
		pic, err := read(context.Background())
		return pasted{pic: pic, err: err, quiet: quiet}
	}
}

// onPasted attaches the picture a paste brought: it goes with the next
// message, standing in the text where the cursor was.
func (m model) onPasted(msg pasted) (tea.Model, tea.Cmd) {
	switch {
	case errors.Is(msg.err, clipboard.ErrNoImage):
		m.notice = ""
		if !msg.quiet {
			m.notice = i18n.T("chat.image.none")
		}
		return m, nil
	case msg.err != nil:
		m.notice = i18n.T("chat.image.failed", msg.err.Error())
		return m, nil
	}
	m.imageSeq++
	m.images = append(m.images, attached{n: m.imageSeq, pic: msg.pic})
	m.input.Insert(imageToken(m.imageSeq) + " ")
	m.notice = i18n.T("chat.image.attached", m.imageSeq, sizeOf(msg.pic))
	return m, nil
}

// sizeOf is a picture's size as the person weighs it: its pixels and about
// how many tokens it adds to the message.
func sizeOf(p clipboard.Picture) string {
	if p.Width == 0 {
		return fmt.Sprintf("%d kB", (len(p.Data)+512)/1024)
	}
	return fmt.Sprintf("%d×%d · ~%s tokens", p.Width, p.Height, tokenCount(p.Width*p.Height/750))
}

// takeImages hands the turn the pictures still standing in the text.
func (m *model) takeImages(text string) {
	m.outgoing = nil
	for _, a := range m.images {
		if strings.Contains(text, imageToken(a.n)) {
			m.outgoing = append(m.outgoing, engine.Image{MediaType: a.pic.MediaType, Data: a.pic.Data})
		}
	}
	m.images = nil
}

// interrupt stops the turn, and denies whatever it was waiting on so no hook
// is left blocking the engine.
func (m *model) interrupt() {
	if m.opts.Approvals != nil {
		m.opts.Approvals.DenyAll(i18n.T("chat.approve.interrupted"))
	}
	m.asking, m.ask = nil, asked{}
	m.engine().Cancel()
}

// submit routes what was typed: a command of the chat's own, the shell, or
// the engine — which also receives every /skill the chat does not know.
func (m model) submit(text string) (tea.Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}
	m.takeImages(text)
	m.input.Reset()
	m.menu, m.menuOff = nil, false
	m.notice = ""
	if len(m.history) == 0 || m.history[len(m.history)-1] != text {
		m.history = append(m.history, text)
	}
	m.histPos = len(m.history)
	k := m.look
	echo := say("\n" + k.userLine(text, m.width))
	note := func(style lipgloss.Style, lines ...string) (tea.Model, tea.Cmd) {
		return m, tea.Sequence(echo, say(k.nested(lines, style)))
	}

	switch {
	case text == "/exit" || text == "/quit":
		m.leaving = true
		return m, nil
	case text == "/help":
		return m, tea.Sequence(echo, say(m.helpBlock()))
	case text == "/clear":
		m.stopPlan()
		m.replaceSession("")
		return note(k.dim, i18n.T("chat.cleared"))
	case text == "/model" || strings.HasPrefix(text, "/model "):
		return m, tea.Sequence(echo, say(m.switchModel(strings.TrimSpace(strings.TrimPrefix(text, "/model")))))
	case text == "/resume" || strings.HasPrefix(text, "/resume "):
		return m, tea.Sequence(echo, say(m.resume(strings.TrimSpace(strings.TrimPrefix(text, "/resume")))))
	case text == "/cost":
		return m, tea.Sequence(echo, say(m.costBlock()))
	case text == "/audit":
		block, fixes := m.auditBlock()
		m.fixes = fixes
		return m, tea.Sequence(echo, say(block))
	case strings.HasPrefix(text, "/audit "):
		// A fix is a request to the agent: it goes as one, the plan left out.
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/audit "))
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(m.fixes) {
			return note(k.warn, i18n.T("chat.audit.unknown", arg))
		}
		fix := m.fixes[n-1]
		m.stopPlan()
		return m.sendToEngine(fix.Prompt, tea.Sequence(echo, say(k.nested([]string{i18n.T("chat.audit.asking", fix.Label)}, k.dim))))
	case text == "/runs":
		lines := watch.Live(m.opts.Root).Lines(m.width-1, 40)
		if len(lines) == 0 {
			return note(k.dim, i18n.T("chat.runs.none"))
		}
		return m, tea.Sequence(echo, say("\n"+strings.Join(lines[:len(lines)-1], "\n")))
	case text == "/watch":
		if m.opts.Gofi == nil {
			return note(k.warn, i18n.T("chat.gofi.unavailable"))
		}
		return m, tea.Sequence(echo, tea.ExecProcess(m.opts.Gofi("watch"), func(err error) tea.Msg { return watchClosed{err} }))
	case strings.HasPrefix(text, "!"):
		command := strings.TrimSpace(text[1:])
		if command == "" {
			return m, echo
		}
		m.running, m.started, m.frame = true, time.Now(), 0
		return m, tea.Batch(echo, runShell(m.opts.Root, command), tickCmd())
	case text == "/brief":
		if m.plan == nil || m.plan.res == nil {
			return note(k.dim, i18n.T("chat.plan.no_brief"))
		}
		return m, tea.Sequence(echo, say("\n"+k.markdown(m.plan.res.Brief, m.width)))
	case strings.HasPrefix(text, "/send "):
		m.stopPlan()
		return m.sendToEngine(strings.TrimSpace(strings.TrimPrefix(text, "/send ")), echo)
	case m.wait != waitNone:
		return m.onPlanReply(text, echo)
	}
	// A question the index answers is asked of the index: gofi reads the
	// project and no model is called.
	if args, ok := gofiArgs(text); ok {
		if m.opts.Gofi == nil {
			return note(k.warn, i18n.T("chat.gofi.unavailable"))
		}
		if len(args) == 1 {
			return note(k.warn, i18n.T("chat.gofi.usage", text))
		}
		m.running, m.started, m.frame = true, time.Now(), 0
		return m, tea.Batch(echo, runGofi(m.opts.Gofi(args...), args), tickCmd())
	}
	// A message with a picture goes as it is: the plan is made of text.
	if !strings.HasPrefix(text, "/") && m.opts.Intake != nil && len(m.outgoing) == 0 {
		return m.startIntake(text, echo)
	}
	return m.sendToEngine(text, echo)
}

func (m model) onEvent(e engine.Event) (tea.Model, tea.Cmd) {
	next := waitEvent(m.events)
	k := m.look
	switch ev := e.(type) {
	case engine.Meta:
		if ev.Model != "" {
			m.modelN = ev.Model
		}
		m.describe(ev)
		return m, next

	case engine.Usage:
		// What the last answer read is what the conversation holds: every
		// turn from here reads it again. A subagent's is its own.
		if held := ev.Input + ev.CacheRead + ev.CacheWrite; !ev.Total && !ev.Sub && held > 0 {
			m.ctxTokens = held + ev.Output
		}
		// Every message is billed, a subagent's too; the turn's total
		// already holds them.
		if !ev.Total {
			m.ledger.Usage(ev.Input, ev.Output, ev.CacheRead, ev.CacheWrite)
		}
		return m, next

	case engine.Delivered:
		// The model took a message sent during the turn: it joins the
		// conversation here, where it was read.
		for i, st := range m.steered {
			if st.id != ev.ID {
				continue
			}
			m.steered = append(m.steered[:i:i], m.steered[i+1:]...)
			m.lastFailed = false
			return m, tea.Sequence(m.flushLive(), say("\n"+k.userLine(st.text, m.width)), next)
		}
		return m, next

	case engine.Delta:
		// A subagent writes to the agent that asked, not to the person.
		if ev.Sub {
			return m, next
		}
		if ev.Kind == engine.DeltaThinking {
			m.think = true
			return m, next
		}
		m.think = false
		m.live += ev.Text
		return m, next

	case engine.Message:
		// What the agents read goes into the ledger, whoever read it.
		for _, b := range ev.Blocks {
			if b.Kind == engine.BlockToolUse {
				m.ledger.Call(b.ID, b.Name, b.Input)
			}
		}
		// A subagent's steps are followed on the panel; what it found comes
		// back as the result of the call that started it.
		if ev.Parent != "" {
			for _, b := range ev.Blocks {
				if b.Kind == engine.BlockToolUse {
					m.noteCall(b, ev.Parent)
				}
			}
			return m, next
		}
		// The finished message is authoritative: its text replaces what was
		// streamed, and its tool calls appear only here.
		var out []string
		for _, b := range ev.Blocks {
			switch b.Kind {
			case engine.BlockText:
				out = append(out, k.markdown(b.Text, m.width))
			case engine.BlockToolUse:
				m.tools[b.ID] = b.Name
				m.noteCall(b, "")
				m.lastCall = b.ID
				summary := toolSummary(b.Input, m.opts.Root, m.width-len(b.Name)-6)
				call := k.bold.Render(b.Name)
				if summary != "" {
					call += k.dim.Render("(") + summary + k.dim.Render(")")
				}
				out = append(out, k.accent.Render("● ")+call)
				m.calls[b.ID] = ansi.Strip(call)
			}
		}
		m.live = ""
		if len(out) == 0 {
			return m, next
		}
		return m, tea.Sequence(say("\n"+strings.Join(out, "\n\n")), next)

	case engine.ToolResult:
		m.ledger.Result(ev.ToolUseID, ev.Text, ev.IsError)
		if ev.Parent != "" {
			return m, next
		}
		m.noteResult(ev.ToolUseID)
		style := k.dim
		if ev.IsError {
			style = k.bad
		}
		lines := preview(ev.Text, 4, func(n int) string { return i18n.T("chat.more_lines", n) })
		if len(lines) == 0 {
			lines = []string{i18n.T("chat.no_output")}
		}
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], m.width-6, "…")
		}
		block := k.nested(lines, style)
		// Calls made side by side answer out of order: a result that does
		// not follow its call names it.
		if call := m.calls[ev.ToolUseID]; call != "" && ev.ToolUseID != m.lastCall {
			block = k.dim.Render("  "+ansi.Truncate(call, m.width-4, "…")) + "\n" + block
		}
		m.lastCall = ""
		return m, tea.Sequence(say(block), next)

	case engine.Done:
		m.lastDone = ev
		m.ledger.Turn()
		cost := m.account(ev)
		var line string
		switch {
		case ev.Cancelled:
			m.lastFailed = true
			line = k.nested([]string{i18n.T("chat.interrupted")}, k.warn)
		case ev.IsError:
			m.lastFailed = true
			line = "\n" + k.bullet(k.bad, ev.Err, k.text, m.width)
		default:
			line = k.dim.Render("  " + turnFooter(ev, cost))
			if tip := m.contextTip(); tip != "" {
				line += "\n" + k.dim.Render("  "+tip)
			}
		}
		return m, tea.Sequence(m.flushLive(), say(line), next)

	case engine.Failure:
		m.lastFailed = true
		out := "\n" + k.bullet(k.bad, ev.Message, k.text, m.width)
		if ev.Hint != "" {
			out += "\n" + k.nested([]string{ev.Hint}, k.dim)
		}
		return m, tea.Sequence(say(out), next)
	}
	return m, next
}

// flushLive prints streamed text that never got its finished message, so a
// cut-off answer is not lost from the scrollback.
func (m *model) flushLive() tea.Cmd {
	text := strings.TrimSpace(m.live)
	m.live = ""
	if text == "" {
		return nil
	}
	return say("\n" + m.look.markdown(text, m.width))
}

// turnFooter is what a turn took: its time and what it cost.
func turnFooter(d engine.Done, cost float64) string {
	parts := []string{fmt.Sprintf("%.1fs", d.Duration.Seconds())}
	if cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.3f", cost))
	}
	return strings.Join(parts, " · ")
}

// account takes a finished turn into what the conversation spent and returns
// the turn's own cost. The engine reports its running total since it started:
// the chat's session lives across turns, so a turn cost what the total grew
// by; a phase's session is new, and its total is the phase's.
func (m *model) account(d engine.Done) float64 {
	if d.ContextWindow > 0 {
		m.ctxWindow = d.ContextWindow
	}
	cost := d.CostUSD
	if !m.conducting {
		if d.CostUSD >= m.lastCost {
			cost = d.CostUSD - m.lastCost
		}
		m.lastCost = d.CostUSD
	}
	m.spent += cost
	return cost
}

// contextSteps are the sizes of a conversation at which the person is told
// what it costs to carry: each turn reads it all again.
var contextSteps = []int{100_000, 200_000, 400_000, 800_000}

// contextTip says, once per size reached, that the conversation grew and how
// to make it small again. Empty otherwise.
func (m *model) contextTip() string {
	if m.conducting {
		return ""
	}
	step := 0
	for _, s := range contextSteps {
		if m.ctxTokens >= s {
			step = s
		}
	}
	if step <= m.ctxWarned {
		if m.ctxTokens < contextSteps[0] {
			m.ctxWarned = 0
		}
		return ""
	}
	m.ctxWarned = step
	return i18n.T("chat.context.tip", tokenCount(m.ctxTokens))
}

// tokenCount is a number of tokens at a glance: 950, 42k, 1.2M.
func tokenCount(n int) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1_000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	}
	return fmt.Sprint(n)
}

// meter is the conversation at a glance, for the status line: the model, how
// much it holds, what it has cost. It gives way from the right when the line
// is short.
func (m model) meter(room int) string {
	parts := []string{m.modelN}
	if m.ctxTokens > 0 {
		held := "ctx " + tokenCount(m.ctxTokens)
		if m.ctxWindow > 0 {
			held += fmt.Sprintf(" (%d%%)", min(m.ctxTokens*100/m.ctxWindow, 100))
		}
		parts = append(parts, held)
	}
	if m.spent > 0 {
		parts = append(parts, money(m.spent))
	}
	for n := len(parts); n > 0; n-- {
		if s := strings.Join(parts[:n], " · "); ansi.StringWidth(s) <= room {
			return s
		}
	}
	return ""
}

// money is an amount spent, with the cents of a cent while it is small.
func money(usd float64) string {
	if usd < 0.1 {
		return fmt.Sprintf("$%.3f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

// auditBlock is /audit: what the conversation read and what that cost, what
// the numbers say is worth changing, and the documents of the project that
// cannot be read cheaply — each with the fix /audit <n> asks the agent for.
// It reads the ledger and the disk: no model is called to draw it.
func (m model) auditBlock() (string, []ledger.Fix) {
	k := m.look
	r := m.ledger.Report()
	t := r.Tokens
	lines := []string{i18n.T("chat.audit.tokens", ledger.Count(t.Input), ledger.Count(t.CacheRead), ledger.Count(t.CacheWrite), ledger.Count(t.Output))}
	if r.Calls == 0 {
		lines = append(lines, k.dim.Render(i18n.T("chat.audit.none")))
	} else {
		lines = append(lines, i18n.T("chat.audit.retrieval", r.Calls, ledger.Count(r.Retrieved), r.Graph, r.Code, r.Docs))
		var top []string
		for _, row := range r.Heaviest {
			if row.Tokens > 0 {
				top = append(top, strings.TrimSpace(row.Name+" "+row.Target)+" ~"+ledger.Count(row.Tokens))
			}
		}
		if len(top) > 0 {
			// Three is enough to say where the context went.
			for _, l := range wrapLines(i18n.T("chat.audit.heaviest", strings.Join(top[:min(len(top), 3)], " · ")), m.width-8) {
				lines = append(lines, k.dim.Render(l))
			}
		}
	}
	var fixes []ledger.Fix
	var repairs []string
	mark := map[ledger.Level]string{ledger.Warn: k.warn.Render("!"), ledger.Info: k.dim.Render("·"), ledger.Good: k.good.Render("✓")}
	wrap := func(lead, text string) []string {
		out := wrapLines(text, m.width-8-ansi.StringWidth(lead))
		for i := range out {
			if i == 0 {
				out[i] = lead + " " + out[i]
			} else {
				out[i] = strings.Repeat(" ", ansi.StringWidth(lead)+1) + out[i]
			}
		}
		return out
	}
	for _, f := range append(r.Findings, ledger.Scan(m.opts.Root, r.Findings)...) {
		if f.Fix == nil {
			lines = append(lines, wrap(mark[f.Level], f.Text)...)
			continue
		}
		fixes = append(fixes, *f.Fix)
		repairs = append(repairs, wrap(k.accent.Render(fmt.Sprintf("%d.", len(fixes))), f.Text+" → "+f.Fix.Label)...)
	}
	if len(repairs) > 0 {
		lines = append(append(lines, "", k.bold.Render(i18n.T("chat.audit.fixes"))), repairs...)
	}
	return "\n" + k.bullet(k.accent, i18n.T("chat.audit.title"), k.bold, m.width) + "\n" + k.nested(lines, k.text), fixes
}

// costBlock is /cost: what the conversation holds and what it has spent,
// from what the engine already reported — nothing is asked of a model.
func (m model) costBlock() string {
	k := m.look
	held := i18n.T("chat.cost.empty")
	if m.ctxTokens > 0 {
		held = tokenCount(m.ctxTokens) + " tokens"
		if m.ctxWindow > 0 {
			held += fmt.Sprintf(" / %s (%d%%)", tokenCount(m.ctxWindow), min(m.ctxTokens*100/m.ctxWindow, 100))
		}
	}
	lines := []string{
		i18n.T("chat.cost.context", held),
		i18n.T("chat.cost.spent", m.spent),
		k.dim.Render(i18n.T("chat.cost.hint")),
	}
	return "\n" + k.bullet(k.accent, i18n.T("chat.cost.title"), k.bold, m.width) + "\n" + k.nested(lines, k.text)
}

// liveLines is the answer being written and how long the turn has taken:
// what follows the conversation until it is finished and joins it.
func (m model) liveLines() []string {
	if !m.running {
		return nil
	}
	k := m.look
	var out []string
	if text := strings.TrimSpace(m.live); text != "" {
		lines := strings.Split(k.markdown(text, m.width), "\n")
		// Keep the live answer to the last lines that fit: the whole of it
		// reaches the conversation once it is finished.
		keep := 12
		if m.pinned {
			keep = min(max(m.height/4, 3), keep)
		}
		if len(lines) > keep {
			lines = lines[len(lines)-keep:]
		}
		out = append(append(out, ""), lines...)
	}
	label := i18n.T("chat.working")
	switch {
	case m.planning:
		label = i18n.T("chat.planning")
	case m.think:
		label = i18n.T("chat.thinking")
	}
	secs := int(time.Since(m.started).Seconds())
	spin := string(spinFrames[m.frame%len(spinFrames)])
	hint := i18n.T("chat.esc_hint")
	if m.events != nil {
		hint += " · " + i18n.T("chat.steer.enter")
	}
	status := ansi.Truncate(spin+" "+label+" ("+elapsed(secs)+" · "+hint+")", max(m.width-1, 10), "…")
	return append(out, "", k.accent.Render(status))
}

// promptLines is the prompt and what goes with it: the messages waiting for
// the agent's next step, the prompt itself — or, while the agent waits for a
// yes, the dialog in its place —, and under it the menu or the status line.
func (m model) promptLines() []string {
	k := m.look
	out := m.steerLines()
	asking := len(m.asking) > 0
	rule := k.border.Render(strings.Repeat("─", max(m.width, 10)))
	if asking {
		limit := 40
		if m.height > 0 {
			limit = m.height - len(out) - 6
			if m.pinned {
				limit -= len(m.panel(m.panelLimit()))
			}
		}
		out = append(append(out, ""), m.approvalView(m.asking[0], max(limit, 12))...)
	} else {
		in := m.input
		in.SetWidth(m.width - 3)
		rows := 10
		if m.height > 0 {
			rows = min(max(m.height/3, 3), rows)
		}
		out = append(out, "", rule)
		out = append(append(out, in.View(k, m.color, rows)...), rule)
	}

	if m.menu != nil && !asking {
		return append(out, m.menu.view(k, m.width)...)
	}
	status := i18n.T("chat.keys")
	switch {
	case asking && m.ask.typing:
		status = i18n.T("chat.approve.note_keys")
	case asking:
		status = i18n.T("chat.approve.keys")
	case m.wait == waitAnswer || m.wait == waitElicit:
		status = i18n.T("chat.plan.answer_hint")
	case m.wait == waitConfirm || m.wait == waitContinue:
		status = i18n.T("chat.plan.confirm_keys")
	}
	if m.notice != "" {
		status = m.notice
	}
	// The meter keeps its place on the right; a long hint gives way to it.
	right := m.meter(max(m.width/2, 24))
	room := m.width - 3
	if right != "" {
		room -= ansi.StringWidth(right) + 2
	}
	if room < 12 {
		right, room = "", m.width-3
	}
	status = ansi.Truncate(status, max(room, 8), "…")
	footer := "  " + status
	if right != "" {
		footer += strings.Repeat(" ", max(m.width-3-ansi.StringWidth(status)-ansi.StringWidth(right), 2)) + right
	}
	footer = ansi.Truncate(footer, max(m.width-1, 10), "…")
	return append(out, k.dim.Render(footer))
}

var spinFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tick{} })
}

// waitEvent reads the next event of the running turn.
func waitEvent(ch <-chan engine.Event) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return turnEnded{}
		}
		return eventMsg{e}
	}
}

func runShell(dir, command string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", command)
		} else {
			cmd = exec.Command("sh", "-c", command)
		}
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return shellDone{command: command, output: string(out), err: err}
	}
}

// runGofi asks gofi itself, in the project: the index's answer is read from
// disk, and it is the whole of it that matters — most of it is shown.
func runGofi(cmd *exec.Cmd, args []string) tea.Cmd {
	return func() tea.Msg {
		out, err := cmd.CombinedOutput()
		return shellDone{command: "gofi " + strings.Join(args, " "), output: string(out), err: err, keep: 40}
	}
}

func (m model) shellBlock(r shellDone) string {
	k := m.look
	dot := k.good
	if r.err != nil {
		dot = k.bad
	}
	name, keep := "Shell", 8
	if r.keep > 0 {
		name, keep = "gofi", r.keep
		r.command = strings.TrimPrefix(r.command, "gofi ")
	}
	head := dot.Render("● ") + k.bold.Render(name) + k.dim.Render("(") + ansi.Truncate(r.command, m.width-12, "…") + k.dim.Render(")")
	lines := preview(r.output, keep, func(n int) string { return i18n.T("chat.more_lines", n) })
	if len(lines) == 0 {
		lines = []string{i18n.T("chat.no_output")}
	}
	if r.err != nil {
		lines = append(lines, r.err.Error())
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], m.width-6, "…")
	}
	return "\n" + head + "\n" + k.nested(lines, k.dim)
}

func (m model) helpBlock() string {
	k := m.look
	var rows [][2]string
	for _, c := range append(chatCommands(), gofiCommands()...) {
		rows = append(rows, [2]string{strings.TrimSpace("/" + c.name + " " + c.hint), c.desc})
	}
	rows = append(rows,
		[2]string{"!<command>", i18n.T("chat.help.shell")},
		[2]string{"@<path>", i18n.T("chat.help.tab")},
		[2]string{"alt+enter", i18n.T("chat.help.newline")},
		[2]string{"enter", i18n.T("chat.help.steer")},
		[2]string{"ctrl+v", i18n.T("chat.help.image")},
		[2]string{"esc", i18n.T("chat.help.esc")},
		[2]string{"↑ ↓", i18n.T("chat.help.history")},
	)
	var lines []string
	for _, r := range rows {
		pad := strings.Repeat(" ", max(18-ansi.StringWidth(r[0]), 1))
		lines = append(lines, k.accent.Render(r[0])+pad+r[1])
	}
	lines = append(lines, "", k.dim.Render(i18n.T("chat.help.engine")))
	if skills := gofiSkills(m.skills); len(skills) > 0 {
		lines = append(lines, "", i18n.T("chat.help.skills"))
		for _, s := range skills {
			lines = append(lines, k.accent.Render("/"+s))
		}
	}
	return "\n" + k.bullet(k.accent, i18n.T("chat.help.title"), k.bold, m.width) + "\n" + k.nested(lines, k.text)
}

// gofiSkills keeps the project's own skills out of the engine's full list of
// slash commands, which also carries its built-ins.
func gofiSkills(all []string) []string {
	var out []string
	for _, s := range all {
		if strings.HasPrefix(s, "gofi-") {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// Delivery is the loop carrying out what a plan builds: its events as they
// happen, how it ended, and how to stop it — at its next checkpoint, so it
// goes on with gofi run --resume.
type Delivery struct {
	Events <-chan loop.Event
	Wait   func() error
	Stop   func()
}
