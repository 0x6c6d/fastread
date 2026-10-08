//go:build !nogui && cgo

package gui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/0x6c6d/fastread/internal/state"
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
	var once sync.Once
	done := func(err error) {
		once.Do(func() { finish(p.Index(), err) })
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done(fmt.Errorf("gui: internal error: %v", r))
			}
		}()
		done(loop(ctx, p))
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

// loop runs the window event loop until quit, close, end of text or ctx done.
func loop(ctx context.Context, p *state.Player) error {
	shaper, err := newShaper()
	if err != nil {
		return err
	}
	w := new(app.Window)
	w.Option(app.Title("fastread"), app.Size(unit.Dp(600), unit.Dp(300)))

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			w.Invalidate()
		case <-stop:
		}
	}()

	var ops op.Ops
	tag := new(int)
	started := false
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if quitKey(gtx, tag) {
				p.Apply(state.ActQuit)
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			if !started {
				p.Start()
				started = true
			}
			if p.Tick() {
				return nil
			}
			draw(gtx, shaper, p, tag)
			gtx.Execute(op.InvalidateCmd{At: p.Deadline()})
			e.Frame(gtx.Ops)
		}
	}
}

// quitKey drains key events for tag and reports whether q, Esc or Ctrl+C was pressed.
func quitKey(gtx layout.Context, tag event.Tag) bool {
	quit := false
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: "Q"},
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: "C", Required: key.ModCtrl},
		)
		if !ok {
			return quit
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
			switch {
			case ke.Name == "Q", ke.Name == key.NameEscape,
				ke.Name == "C" && ke.Modifiers.Contain(key.ModCtrl):
				quit = true
			}
		}
	}
}

// draw fills the window black and draws the current word centred in white.
func draw(gtx layout.Context, shaper *text.Shaper, p *state.Player, tag event.Tag) {
	paint.FillShape(gtx.Ops, color.NRGBA{A: 0xff}, clip.Rect{Max: gtx.Constraints.Max}.Op())
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	area.Pop()

	word := p.Token().Text
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}.Add(gtx.Ops)
	material := macro.Stop()

	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return widget.Label{Alignment: text.Middle, MaxLines: 1}.
			Layout(gtx, shaper, font.Font{}, wordSize(p.Size()), word, material)
	})
}
