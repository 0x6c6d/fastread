//go:build !nogui && cgo

package gui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/opentype"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"

	"github.com/0x6c6d/fastread/internal/state"
)

// Colours of everything the window draws; red is used for the focus cluster only.
var (
	colBlack = color.NRGBA{A: 0xff}
	colWhite = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colRed   = color.NRGBA{R: 0xff, A: 0xff}
	colBar   = color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xff}
	colGrey  = color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}
)

// sizeSp maps size levels 1..5 to text sizes.
var sizeSp = [...]unit.Sp{20, 32, 48, 64, 96}

func wordSize(level int) unit.Sp {
	if level < 1 {
		level = 1
	}
	if level > len(sizeSp) {
		level = len(sizeSp)
	}
	return sizeSp[level-1]
}

// Run opens the fastread window. Without DISPLAY and WAYLAND_DISPLAY it returns ErrNoDisplay
// synchronously. Otherwise it hands the main thread to Gio (app.Main never returns on Linux);
// the window goroutine reports the outcome exactly once through finish.
func Run(ctx context.Context, p *state.Player, opts Options, finish func(last int, err error)) error {
	if !hasDisplay(opts.getenv()) {
		return ErrNoDisplay
	}
	s := newSession(p, finish)
	go func() {
		defer s.guard()
		loop(ctx, p, s)
	}()
	app.Main()
	return nil
}

func newShaper() (*text.Shaper, error) {
	faces, err := opentype.ParseCollection(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("gui: font: %w", err)
	}
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces)), nil
}

// loop runs the window event loop until quit, close, end of text or ctx done; every end
// goes through s, and loop returns right after it. It is the only code that touches the
// Player while the window is open.
func loop(ctx context.Context, p *state.Player, s *session) {
	m, err := newGioMeasurer()
	if err != nil {
		s.end(err)
		return
	}
	w := new(app.Window)
	w.Option(app.Title("fastread"), app.Size(unit.Dp(600), unit.Dp(300)))

	// ctx helper: wake the window on ctx done so the loop below ends the session; it
	// also ends when the session does.
	go func() {
		select {
		case <-ctx.Done():
			w.Invalidate()
		case <-s.doneCh():
		}
	}()

	var ops op.Ops
	started := false
	for !s.ended() {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			s.end(windowErr(e.Err))
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			// a. keys
			for {
				ev, ok := gtx.Event(keyFilters()...)
				if !ok {
					break
				}
				ke, ok := ev.(key.Event)
				if !ok {
					continue
				}
				if a, ok := keyAction(ke); ok && p.Apply(a) {
					s.end(nil)
					return
				}
			}
			if ctx.Err() != nil {
				s.end(nil)
				return
			}
			// b. clock
			if !started {
				p.Start()
				started = true
			}
			for p.Playing() && !time.Now().Before(p.Deadline()) {
				if p.Tick() {
					s.end(nil)
					return
				}
			}
			// c. layout, d. draw
			draw(gtx.Ops, m, p, e)
			// e. next frame at the absolute deadline
			if p.Playing() {
				gtx.Execute(op.InvalidateCmd{At: p.Deadline()})
			}
			e.Frame(gtx.Ops)
		}
	}
}

// draw lays out the current word for this frame's size and paints the whole window.
func draw(ops *op.Ops, m *gioMeasurer, p *state.Player, e app.FrameEvent) {
	in := LayoutInput{
		Word: p.Token().Text, Level: p.Size(), W: e.Size.X, H: e.Size.Y,
		PxPerSp: e.Metric.PxPerSp, Part: p.Part(), M: m,
	}
	res := Layout(in)
	p.SetParts(res.Parts)
	if p.Part() != res.Part {
		in.Part = p.Part()
		res = Layout(in)
	}

	size := image.Pt(max(e.Size.X, 1), max(e.Size.Y, 1))
	paint.FillShape(ops, colBlack, clip.Rect{Max: size}.Op())

	x := res.OriginX
	for i, c := range res.Clusters {
		col := colWhite
		if i == res.Focus {
			col = colRed
		}
		drawText(ops, m, c, res.Px, x, res.BaselineY, col)
		if i < len(res.Advances) {
			x += res.Advances[i]
		}
	}
	paint.FillShape(ops, colWhite, clip.Rect(res.TickTop).Op())
	paint.FillShape(ops, colWhite, clip.Rect(res.TickBottom).Op())

	if !p.ShowHelp() && !p.ShowProgress() {
		return
	}
	ch := LayoutChrome(e.Size.X, e.Size.Y, e.Metric.PxPerSp, p.Index(), p.Len(), m)
	if p.ShowHelp() {
		drawText(ops, m, HelpText, ch.Px, fixed.I(ch.Help.X), fixed.I(ch.Help.Y), colWhite)
	}
	if p.ShowProgress() {
		paint.FillShape(ops, colBar, clip.Rect(ch.Bar).Op())
		paint.FillShape(ops, colWhite, clip.Rect(ch.BarFill).Op())
		wpm, ok := p.EffectiveWPM()
		txt := ProgressText(p.Index(), p.Len(), wpm, ok)
		drawText(ops, m, txt, ch.Px, fixed.I(ch.Progress.X), fixed.I(ch.Progress.Y), colGrey)
	}
}

// drawText fills the outline of s shaped at px with col, its pen at (x, baseline y).
func drawText(ops *op.Ops, m *gioMeasurer, s string, px, x, y fixed.Int26_6, col color.NRGBA) {
	v := m.shape(s, px)
	if len(v.glyphs) == 0 {
		return
	}
	t := op.Affine(f32.AffineId().Offset(f32.Pt(float32(x)/64, float32(y)/64))).Push(ops)
	paint.FillShape(ops, col, clip.Outline{Path: m.sh.Shape(v.glyphs)}.Op())
	t.Pop()
}
