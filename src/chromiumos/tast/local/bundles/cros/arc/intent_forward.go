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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IntentForward,
		Desc:         "Checks Android intents are forwarded to Chrome",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
	})
}

func IntentForward(ctx context.Context, s *testing.State) {
	const (
		viewAction          = "android.intent.action.VIEW"
		viewDownloadsAction = "android.intent.action.VIEW_DOWNLOADS"
		setWallpaperAction  = "android.intent.action.SET_WALLPAPER"

		filesAppURL        = "chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.html"
		wallpaperPickerURL = "chrome-extension://obklkkbkpaoaejdabbfldmcfplpdgolj/main.html"
	)

	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	if err := a.WaitIntentHelper(ctx); err != nil {
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

		testing.ContextLogf(ctx, "Testing: %s(%s) -> %s", action, data, url)

		if err := a.SendIntentCommand(ctx, action, data).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to send an intent %q: %v", action, err)
			return
		}

		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return t.URL == url
		})
		if err != nil {
			s.Errorf("%s(%s) -> %s: %v", action, data, url, err)
		} else {
			conn.Close()
		}
	}

	checkIntent(viewAction, localWebURL, localWebURL)
	checkIntent(viewDownloadsAction, "", filesAppURL)
	checkIntent(setWallpaperAction, "", wallpaperPickerURL)
}
