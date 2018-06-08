// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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
	_, err := cr.NewConnForTarget(newCtx, func(t *chrome.Target) bool {
		return t.URL == url
	})
	if err != nil {
		// HACK: Print existing targets when we could not find the desired one
		// for debugging purpose.
		testing.ContextLog(ctx, "Timeout expired on waiting for a tab. Existing tabs:")
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

		sampleWebURL       = "https://www.google.com/humans.txt"
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

	arc.SendIntent(viewAction, sampleWebURL)
	err = waitForTab(s.Context(), cr, sampleWebURL)
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
