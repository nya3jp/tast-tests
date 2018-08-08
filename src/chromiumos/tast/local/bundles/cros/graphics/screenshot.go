// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Screenshot,
		Desc: "Takes a screenshot",
		Attr: []string{"informational"},
		// The screenshot tool requires a display to be connected. We use the
		// presence of an internal display backlight as a proxy.
		SoftwareDeps: []string{"chrome_login", "display_backlight"},
	})
}

func Screenshot(s *testing.State) {
	const screenshotName = "screenshot.png"

	defer faillog.SaveIfError(s)

	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Show a page with orange background.
	const html = "<style>body { background-color: #c84; }</style>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
  chrome.windows.getLastFocused({}, (window) => {
    chrome.windows.update(window.id, {state: 'maximized'}, resolve);
  });
})
`, nil); err != nil {
		s.Fatal("Maximizing the window failed: ", err)
	}

	// verify takes a screenshot and checks if orange pixels fill up more than half of the screen.
	verify := func() bool {
		path := filepath.Join(s.OutDir(), screenshotName)
		cmd := testexec.CommandContext(ctx, "screenshot", "--internal", path)
		if err := cmd.Run(); err != nil {
			// We do not abort here because:
			// - screenshot command might have failed just because the internal display is not on yet
			// - Context deadline might be reached while taking a screenshot, which should be
			//   reported as "Screenshot does not contain expected pixels" rather than
			//   "screenshot command failed".
			cmd.DumpLog(ctx)
			return false
		}

		f, err := os.Open(path)
		if err != nil {
			s.Fatal("Failed opening the screenshot image: ", err)
		}
		defer f.Close()

		im, err := png.Decode(f)
		if err != nil {
			s.Fatal("Failed decoding the screenshot image: ", err)
		}

		type Color struct{ r, g, b uint32 }

		getPopularColor := func(im image.Image) (color Color, ratio float64) {
			counter := map[Color]int{}
			box := im.Bounds()
			for x := box.Min.X; x < box.Max.X; x++ {
				for y := box.Min.Y; y < box.Max.Y; y++ {
					r, g, b, _ := im.At(x, y).RGBA()
					counter[Color{r, g, b}] += 1
				}
			}

			best := 0
			for c, cnt := range counter {
				if cnt > best {
					color = c
					best = cnt
				}
			}
			ratio = float64(best) / float64((box.Max.X-box.Min.X)*(box.Max.Y-box.Min.Y))
			return
		}

		color, ratio := getPopularColor(im)

		s.Logf("Most popular color: #%02x%02x%02x (ratio=%v)", color.r/0x101, color.g/0x101, color.b/0x101, ratio)

		near := func(x uint32, y int32) bool {
			// r is allowed color component difference in 16bit value.
			// Most differing color known to the date is #ba8b4a on sumo, so this value should be
			// no less than 0x1212.
			const r = 0x1300
			d := int32(x) - y
			return -r <= d && d <= r
		}
		isOrange := near(color.r, 0xcccc) && near(color.g, 0x8888) && near(color.b, 0x4444)
		return isOrange && ratio >= 0.5
	}

	// Allow up to 10 seconds for the orange screen to render.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if verify() {
			return
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			s.Error("Screenshot does not contain expected pixels. See: ", screenshotName)
			return
		}
	}
}
