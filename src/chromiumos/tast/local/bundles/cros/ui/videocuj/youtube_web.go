// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// YtWeb defines the struct related to youtube web.
type YtWeb struct {
	cr      *chrome.Chrome
	tconn   *chrome.TestConn
	kb      *input.KeyboardEventWriter
	video   videoSrc
	ui      *uiauto.Context
	ytConn  *chrome.Conn
	ytWinID int
	uiHdl   cuj.UIActionHandler

	extendedDisplay bool
}

// NewYtWeb creates an instance of YtWeb.
func NewYtWeb(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, video videoSrc,
	extendedDisplay bool, ui *uiauto.Context, uiHdl cuj.UIActionHandler) *YtWeb {
	return &YtWeb{
		cr:    cr,
		tconn: tconn,
		kb:    kb,
		video: video,
		ui:    ui,
		uiHdl: uiHdl,

		extendedDisplay: extendedDisplay,
	}
}

// OpenAndPlayVideo opens a youtube video on chrome.
func (y *YtWeb) OpenAndPlayVideo(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Open Youtube web")

	// TODO: provide implementation details.
	return nil
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtWeb) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Youtube video fullscreen")

	// TODO: provide implementation details.
	return nil
}

// PauseAndPlayVideo verifies video playback on youtube web.
func (y *YtWeb) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")

	// TODO: provide implementation details.
	return nil
}

// Close closes the resources related to video.
func (y *YtWeb) Close(ctx context.Context) {
	if y.ytConn != nil {
		y.ytConn.CloseTarget(ctx)
		y.ytConn.Close()
		y.ytConn = nil
	}
}
