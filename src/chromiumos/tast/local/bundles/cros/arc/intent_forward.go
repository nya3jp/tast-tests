// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IntentForward,
		Desc:         "Checks Android intents are forwarded to Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      3 * time.Minute,
	})
}

func IntentForward(s *testing.State) {
	const (
		viewAction          = "android.intent.action.VIEW"
		viewDownloadsAction = "android.intent.action.VIEW_DOWNLOADS"
		setWallpaperAction  = "android.intent.action.SET_WALLPAPER"

		filesAppURL        = "chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.html"
		wallpaperPickerURL = "chrome-extension://obklkkbkpaoaejdabbfldmcfplpdgolj/main.html"
	)

	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, cr, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err = a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "It worked!")
	}))
	defer server.Close()
	localWebURL := server.URL + "/" // Must end with a slash

	checkIntent := func(action, data, url string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		testing.ContextLogf(ctx, "Testing: (%s, %s) -> %s", action, data, url)

		cmd := a.SendIntentCommand(ctx, action, data)
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Errorf("Failed to send an intent %q: %v", action, err)
			return
		}

		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return t.URL == url
		})
		if err != nil {
			s.Error(err)
		} else {
			conn.Close()
		}
	}

	checkIntent(viewAction, localWebURL, localWebURL)
	checkIntent(viewDownloadsAction, "", filesAppURL)
	checkIntent(setWallpaperAction, "", wallpaperPickerURL)
}
