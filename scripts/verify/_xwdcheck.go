// Pixel facts for the GUI verification scripts (supervisor-owned; workers never edit it).
// The leading underscore keeps it out of every Go package; scripts/verify/_gui.sh builds it.
//
// xwdcheck reads an XWD window dump (ZPixmap, 32 bpp TrueColor) and prints pixel facts for
// the GUI checks: size, share of black pixels, the box of pure-red pixels and the number of
// "tick rows" (rows whose non-black pixels are exactly the columns cx-1 and cx) above and
// below the red box, and the non-black pixel counts in the top 30 rows (help line) and the
// bottom 40 rows (progress). Usage: xwdcheck <file.xwd>; cx = width/2.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil || len(b) < 100 {
		fmt.Println("error: cannot read xwd")
		os.Exit(2)
	}
	u := func(i int) uint32 { return binary.BigEndian.Uint32(b[4*i:]) }
	hsize, format, w, h := int(u(0)), u(2), int(u(4)), int(u(5))
	order, bpp, bpl := u(7), int(u(11)), int(u(12))
	masks := [3]uint32{u(14), u(15), u(16)}
	off := hsize + 12*int(u(19))
	if format != 2 || bpp != 32 || w <= 0 || h <= 0 || len(b) < off+bpl*h {
		fmt.Println("error: unsupported xwd")
		os.Exit(2)
	}
	pix := func(x, y int) (c [3]uint32) {
		p := b[off+y*bpl+4*x:]
		v := binary.BigEndian.Uint32(p)
		if order == 0 {
			v = binary.LittleEndian.Uint32(p)
		}
		for i, m := range masks {
			s := v
			for m != 0 && m&1 == 0 {
				m >>= 1
				s >>= 1
			}
			c[i] = s & m
		}
		return
	}
	cx := w / 2
	black, red, top, bottom := 0, 0, 0, 0
	rx0, rx1, ry0, ry1 := w, -1, h, -1
	tickRow := make([]bool, h)
	for y := 0; y < h; y++ {
		var cols []int
		for x := 0; x < w; x++ {
			c := pix(x, y)
			if c[0] <= 40 && c[1] <= 40 && c[2] <= 40 {
				black++
				continue
			}
			cols = append(cols, x)
			if y < 30 {
				top++
			}
			if y >= h-40 {
				bottom++
			}
			if c[0] >= 200 && c[1] <= 80 && c[2] <= 80 {
				red++
				rx0, rx1 = min(rx0, x), max(rx1, x)
				ry0, ry1 = min(ry0, y), max(ry1, y)
			}
		}
		tickRow[y] = len(cols) == 2 && cols[0] == cx-1 && cols[1] == cx
	}
	above, below := 0, 0
	for y, t := range tickRow {
		if t && y < ry0 {
			above++
		}
		if t && ry1 >= 0 && y > ry1 {
			below++
		}
	}
	fmt.Printf("size %d %d\n", w, h)
	fmt.Printf("black %d\n", black*100/(w*h))
	fmt.Printf("red %d %d %d %d %d\n", red, rx0, rx1, ry0, ry1)
	fmt.Printf("redcentre2 %d\n", rx0+rx1+1) // twice the ink centre
	fmt.Printf("ticks %d %d\n", above, below)
	fmt.Printf("top %d\n", top)
	fmt.Printf("bottom %d\n", bottom)
}
