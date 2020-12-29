// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"time"

	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	youtubePkg                   = "com.google.android.youtube"
	playerViewID                 = youtubePkg + ":id/player_view"
	qualityListItemID            = youtubePkg + ":id/list_item_text"
	uiWaitTime                   = 15 * time.Second
	waitTimeAfterClickPlayerView = 3 * time.Second
)

var appStartTime time.Duration

// YtApp defines the members related to youtube app.
type YtApp struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	a     *arc.ARC
	d     *androidui.Device
	video Video
	act   *arc.Activity
}

// NewYtApp creates an instance of YtApp.
func NewYtApp(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, a *arc.ARC, d *androidui.Device, video Video) *YtApp {
	return &YtApp{
		tconn: tconn,
		kb:    kb,
		a:     a,
		d:     d,
		video: video,
	}
}

// OpenAndPlayVideo opens a video on youtube app.
func (y *YtApp) OpenAndPlayVideo(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Open Youtube app")

	// TODO: provide implementation details.
	return nil
}

func checkYoutubeAppPIP(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO: provide implementation details.
	return nil
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtApp) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Youtube app fullscreen")

	// TODO: provide implementation details.
	return nil
}

// PauseAndPlayVideo verifies video playback on youtube app.
func (y *YtApp) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")

	// TODO: provide implementation details.
	return nil
}

// Close closes the resources related to video.
func (y *YtApp) Close(ctx context.Context) {
	if y.act != nil {
		y.act.Stop(ctx, y.tconn)
		y.act.Close()
		y.act = nil
	}
}
