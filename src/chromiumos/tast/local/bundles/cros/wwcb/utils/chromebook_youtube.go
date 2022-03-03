// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

// youtube
const (
	VideoTitle = "test video"
	YouTubeURL = "https://youtu.be/Znq6Q-AmCkA"
)

// PlayYouTube open browser to play youtube on chromebook
func PlayYouTube(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	// open chrome to url
	_, err := cr.NewConn(ctx, YouTubeURL, browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "could not get youTube request")
	}

	// check window info is correct
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && strings.Contains(w.Title, VideoTitle) && w.IsVisible == true
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "app window not focused after clicking shelf icon")
	}

	return nil
}

// GetYoutubeWindow get youtube window info
func GetYoutubeWindow(ctx context.Context, tconn *chrome.TestConn) (*ash.Window, error) {
	return ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return strings.Contains(window.Title, VideoTitle)
	})
}

// EnsureYoutubeOnDisplay check youtube is on "the" display
func EnsureYoutubeOnDisplay(ctx context.Context, tconn *chrome.TestConn, wantDisp *display.Info) error {

	// get youtube window
	youtube, err := GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	}

	// ensure window on display
	if err := EnsureWindowOnDisplay(ctx, tconn, youtube.ARCPackageName, wantDisp.ID); err != nil {
		return errors.Wrap(err, "failed to ensure windows on display")
	}

	return nil
}
