package vis

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/dsp"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
)

// feed pushes a loud, bassy, slightly noisy signal into the analyzer.
func feed(an *dsp.Analyzer, t float64) {
	buf := make([][2]float64, 2048)
	for i := range buf {
		x := t + float64(i)/44100
		v := 0.6*math.Sin(2*math.Pi*55*x) + 0.3*math.Sin(2*math.Pi*440*x) + 0.15*math.Sin(2*math.Pi*3000*x) + (rand.Float64()-0.5)*0.1
		buf[i] = [2]float64{v, v * 0.8}
	}
	an.Push(buf)
}

func render(c *Canvas) string {
	var sb strings.Builder
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			ch := c.Cells[y*c.W+x].Ch
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// TestEveryVisualizerDraws runs each style for a few frames on several
// canvas sizes and checks it paints something without panicking.
func TestEveryVisualizerDraws(t *testing.T) {
	an := dsp.NewAnalyzer(2048, 44100)
	opts := OptionsFromConfig(config.Default())
	th := theme.Get("wvfrm")
	dump := os.Getenv("WVFRM_DUMP") != ""
	for _, v := range Registry {
		for _, sz := range [][2]int{{80, 20}, {30, 8}, {3, 2}, {1, 1}} {
			c := NewCanvas(sz[0], sz[1])
			painted := 0
			for frame := 0; frame < 40; frame++ {
				feed(an, float64(frame)*0.05)
				c.Clear()
				v.Draw(c, &Frame{Analyzer: an, Opts: &opts, Theme: th, Time: float64(frame) / 30, Playing: true})
				for _, cell := range c.Cells {
					if cell.Ch != 0 {
						painted++
					}
				}
			}
			if painted == 0 && sz[0] >= 30 {
				t.Errorf("%s painted nothing at %dx%d", v.Name(), sz[0], sz[1])
			}
			if dump && sz[0] == 80 {
				fmt.Printf("=== %s\n%s", v.Name(), render(c))
			}
		}
	}
	if len(Names()) != len(Registry) || Index("matrix") == 0 || Index("nope") != 0 {
		t.Error("registry helpers broken")
	}
}
