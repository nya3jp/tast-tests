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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func openAndPlayNetflixWeb(ctx context.Context, s *testing.State, tconn *chrome.TestConn, cr *chrome.Chrome, kb *input.KeyboardEventWriter, playbackSettings string) (*netflix.Netflix, error) {
	s.Log("Open Netflix web")
	username := s.RequiredVar("ui.netflix_username")
	password := s.RequiredVar("ui.netflix_password")
	n, err := netflix.New(ctx, tconn, username, password, cr, playbackSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open or sign in Netflix")
	}
	s.Log("Go to watch netflix video")
	if err := n.Play(ctx, "https://www.netflix.com/watch/80026431"); err != nil {
		return nil, errors.Wrap(err, "failed to play Netflix video")
	}

	return n, nil
}

func enterNetflixWebFullscreen(ctx context.Context, tconn *chrome.TestConn, nfWinID int) error {
	testing.ContextLog(ctx, "Make Netflix video fullscreen")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	window, err := ash.GetWindow(ctx, tconn, nfWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get specific window")
	} else if window.State == ash.WindowStateFullscreen {
		return errors.New("alreay in fullscreen")
	}

	// Clear notification prompts if exists
	cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Block", Role: ui.RoleTypeButton}, time.Second)
	cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Never", Role: ui.RoleTypeButton}, time.Second)

	if err := kb.Accel(ctx, "F"); err != nil {
		testing.ContextLog(ctx, `kb.Accel(ctx, 'F') return failure : `, err)
		return err
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == nfWinID && w.State == ash.WindowStateFullscreen
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for fullscreen")
	}
	return nil
}

func pauseAndPlayNetflixWeb(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	const (
		playButton  = "Play"
		pauseButton = "Pause"
		timeout     = time.Second * 15
		waitTime    = time.Second * 3
	)
	pauseParams := ui.FindParams{
		Name: pauseButton,
		Role: ui.RoleTypeButton,
	}
	playParams := ui.FindParams{
		Name: playButton,
		Role: ui.RoleTypeButton,
	}

	// Press Tab to show play/pause button
	if err := kb.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to send Tab")
	}

	testing.ContextLog(ctx, "Verify Netflix video is playing")
	if err := ui.WaitUntilExists(ctx, tconn, pauseParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find pause button to check video is playing")
	}

	testing.ContextLog(ctx, "Click pause button")
	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: pauseButton, Role: ui.RoleTypeButton}, timeout); err != nil {
		return errors.Wrap(err, "failed to click pause button")
	}

	testing.ContextLog(ctx, "Verify Netflix video is paused")
	if err := ui.WaitUntilExists(ctx, tconn, playParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find play button to check video is paused")
	}

	// Wait time to see the video is paused
	testing.Sleep(ctx, waitTime)

	testing.ContextLog(ctx, "Click play button")
	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: playButton, Role: ui.RoleTypeButton}, timeout); err != nil {
		return errors.Wrap(err, "failed to click play button")
	}

	testing.ContextLog(ctx, "Verify Netflix video is playing")
	if err := ui.WaitUntilExists(ctx, tconn, pauseParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find pause button to check video is playing")
	}

	// Wait time to see the video is playing
	testing.Sleep(ctx, waitTime)

	return nil
}
