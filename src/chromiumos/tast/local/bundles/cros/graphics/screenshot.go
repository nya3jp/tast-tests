// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Screenshot,
		Desc:         "Takes a screenshot",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Screenshot(s *testing.State) {
	const screenshotName = "screenshot.png"

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
			cmd.DumpLog(ctx)
			s.Fatal("Failed running screenshot command: ", err)
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

		near := func(x uint32, y int32) bool {
			const r = 0x1000
			d := int32(x) - y
			return -r <= d && d <= r
		}

		box := im.Bounds()
		orange := 0
		for x := box.Min.X; x < box.Max.X; x++ {
			for y := box.Min.Y; y < box.Max.Y; y++ {
				r, g, b, _ := im.At(x, y).RGBA()
				if near(r, 0xcccc) && near(g, 0x8888) && near(b, 0x4444) {
					orange++
				}
			}
		}

		total := (box.Max.X - box.Min.X) * (box.Max.Y - box.Min.Y)
		s.Logf("orange ratio = %d / %d = %d%%", orange, total, 100*orange/total)
		return orange >= total/2
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
