// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/screenshot"
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

	if (!screenshot.Screenshot(s, 0.5, screenshot.Color{0xcccc, 0x8888, 0x4444})) {
		s.Fatal("Screenshot did not match expected color")
	}
}
