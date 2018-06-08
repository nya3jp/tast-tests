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

func waitIntentHelper(ctx context.Context) error {
	newCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return arc.WaitSystemEvent(newCtx, "ArcIntentHelperService:ready")
}

func sendIntent(action, data string) error {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return arc.Command("am", args...).Run()
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
	cr, err := chrome.New(s.Context(), chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	if err = waitIntentHelper(s.Context()); err != nil {
		s.Fatal("Failed to wait IntentHelper: ", err)
	}

	sendIntent("android.intent.action.VIEW", "https://www.google.com/humans.txt")
	err = waitForTab(s.Context(), cr, "https://www.google.com/humans.txt")
	if err != nil {
		s.Error("Failed to open a web page: ", err)
	}

	sendIntent("android.intent.action.VIEW_DOWNLOADS", "")
	err = waitForTab(s.Context(), cr, "chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.html")
	if err != nil {
		s.Error("Failed to open Downloads: ", err)
	}

	sendIntent("android.intent.action.SET_WALLPAPER", "")
	err = waitForTab(s.Context(), cr, "chrome-extension://obklkkbkpaoaejdabbfldmcfplpdgolj/main.html")
	if err != nil {
		s.Error("Failed to open wallpaper picker: ", err)
	}
}
