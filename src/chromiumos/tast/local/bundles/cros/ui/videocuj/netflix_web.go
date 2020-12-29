// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/netflix"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	netflixVideo = "https://www.netflix.com/watch/80026431"
)

// NtWeb defines the members related to netflix.
type NtWeb struct {
	tconn            *chrome.TestConn
	cr               *chrome.Chrome
	kb               *input.KeyboardEventWriter
	nfWinID          int
	playbackSettings string
	extendedDisplay  bool
	ui               *uiauto.Context
	ntInstance       *netflix.Netflix
	uiHdl            cuj.UIActionHandler

	username string
	password string
}

// NewNtWeb creates the instance of NtWeb.
func NewNtWeb(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, playbackSettings string, extendedDisplay bool, ui *uiauto.Context, uiHdl cuj.UIActionHandler,
	username, password string) *NtWeb {
	return &NtWeb{
		tconn:            tconn,
		cr:               cr,
		kb:               kb,
		playbackSettings: playbackSettings,
		extendedDisplay:  extendedDisplay,
		ui:               ui,
		uiHdl:            uiHdl,

		username: username,
		password: password,
	}
}

// OpenAndPlayVideo opens a netflix video.
func (n *NtWeb) OpenAndPlayVideo(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Open Netflix web")

	n.ntInstance, err = netflix.New(ctx, n.tconn, n.username, n.password, n.cr, n.uiHdl, n.playbackSettings)
	if err != nil {
		return errors.Wrap(err, "failed to open or sign in Netflix")
	}

	// Clear notification prompts if exists.
	prompts := []string{"Allow", "Never"}
	clearNotificationPrompts(ctx, n.ui, n.uiHdl, prompts...)

	testing.ContextLog(ctx, "Go to watch netflix video")
	if err := n.ntInstance.Play(ctx, netflixVideo); err != nil {
		return errors.Wrap(err, "failed to play Netflix video")
	}

	n.nfWinID, err = getWindowID(ctx, n.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get window ID: ")
	}

	return nil
}

// EnterFullscreen switches netflix video to fullscreen.
func (n *NtWeb) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Netflix video fullscreen")

	window, err := ash.GetWindow(ctx, n.tconn, n.nfWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get specific window")
	} else if window.State == ash.WindowStateFullscreen {
		return errors.New("already in fullscreen")
	}

	ui := n.ui.WithTimeout(5 * time.Second)

	// Clear notification prompts if exists.
	prompts := []string{"Allow", "Never"}
	clearNotificationPrompts(ctx, ui, n.uiHdl, prompts...)

	if err := n.kb.Accel(ctx, "F"); err != nil {
		testing.ContextLog(ctx, `kb.Accel(ctx, 'F') return failure : `, err)
		return err
	}

	if err := waitWindowStateFullscreen(ctx, n.tconn, n.nfWinID); err != nil {
		return errors.Wrap(err, "failed to tap fullscreen button")
	}

	return nil
}

// PauseAndPlayVideo verifies video playback on netflix.
func (n *NtWeb) PauseAndPlayVideo(ctx context.Context) error {
	const (
		playButton  = "Play"
		pauseButton = "Pause"
		timeout     = 15 * time.Second
		waitTime    = 3 * time.Second
	)

	pauseBtn := nodewith.Name(pauseButton).Role(role.Button)
	playBtn := nodewith.Name(playButton).Role(role.Button)

	if err := uiauto.Combine("press tab and find play/pause button to verify video playback",
		n.kb.AccelAction("Tab"),
		n.uiHdl.Click(pauseBtn),
		n.ui.WaitUntilExists(playBtn),
		// Wait time to see the video is paused.
		n.ui.Sleep(time.Second),
		n.uiHdl.Click(playBtn),
		n.ui.WaitUntilExists(pauseBtn),
		// Wait time to see the video is playing.
		n.ui.Sleep(waitTime),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to press tab and find play/pause button")
	}
	return nil
}

// Close closes the resources related to video that .
func (n *NtWeb) Close(ctx context.Context) {
	n.ntInstance.Close(ctx)
}
