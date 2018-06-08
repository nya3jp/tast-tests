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
		Attr:         []string{"bvt"},
		SoftwareDeps: []string{"android", "chrome_login"},
	})
}

func waitForTab(ctx context.Context, cr *chrome.Chrome, url string) error {
	newCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	testing.ContextLogf(ctx, "Waiting for a tab: %s", url)
	_, err := cr.NewConnForTarget(newCtx, func(t *chrome.Target) bool {
		return t.URL == url
	})
	if err != nil {
		// HACK: Print existing targets when we could not find the desired one
		// for debugging purpose.
		testing.ContextLogf(ctx, "Timeout expired on waiting for a tab. Existing tabs:")
		cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			testing.ContextLog(ctx, "  ", t.URL)
			return true
		})
	}
	return err
}

func IntentForward(s *testing.State) {
	const (
		viewAction          = "android.intent.action.VIEW"
		viewDownloadsAction = "android.intent.action.VIEW_DOWNLOADS"
		setWallpaperAction  = "android.intent.action.SET_WALLPAPER"

		filesAppURL        = "chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.html"
		wallpaperPickerURL = "chrome-extension://obklkkbkpaoaejdabbfldmcfplpdgolj/main.html"
	)

	cr, err := chrome.New(s.Context(), chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	if err = arc.WaitArcIntentHelper(s.Context()); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "It worked!")
	}))
	defer server.Close()
	localWebURL := server.URL + "/" // Must end with a slash

	arc.SendIntent(viewAction, localWebURL)
	err = waitForTab(s.Context(), cr, localWebURL)
	if err != nil {
		s.Error("Failed to open a web page: ", err)
	}

	arc.SendIntent(viewDownloadsAction, "")
	err = waitForTab(s.Context(), cr, filesAppURL)
	if err != nil {
		s.Error("Failed to open Downloads: ", err)
	}

	arc.SendIntent(setWallpaperAction, "")
	err = waitForTab(s.Context(), cr, wallpaperPickerURL)
	if err != nil {
		s.Error("Failed to open wallpaper picker: ", err)
	}
}
