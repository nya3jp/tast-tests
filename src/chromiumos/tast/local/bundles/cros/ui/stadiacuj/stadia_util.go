// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stadiacuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// StadiaAllGamesUrl is the url of all Stadia games page.
	StadiaAllGamesUrl = "https://ggp-staging.sandbox.google.com/store/list"
)

// StartGameFromGameListsView locates the game by its name from the game list page
// and starts the game.
func StartGameFromGameListsView(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, n *ui.Node, name string, timeout time.Duration) error {
	gameView := "View " + name + "."
	gamePlay := "Play"
	gameStart := name + " Play game"

	// Find the game view from the game list.
	gameViewButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gameView, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game view button (%s)", gameView)
	}
	defer gameViewButton.Release(ctx)
	if err := gameViewButton.FocusAndWait(ctx, timeout); err != nil {
		return errors.Wrapf(err, "failed to focus on the game view button (%s)", gameView)
	}
	if err := gameViewButton.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click the game view button (%s)", gameView)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for game page to finish loading")
	}

	// Play the game.
	gamePlayButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gamePlay, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game play button (%s)", gamePlay)
	}
	defer gamePlayButton.Release(ctx)
	if err := gamePlayButton.FocusAndWait(ctx, timeout); err != nil {
		return errors.Wrapf(err, "failed to focus on the game play button (%s)", gamePlay)
	}
	if err := gamePlayButton.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click the game play button (%s)", gamePlay)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for game page to finish loading")
	}

	// Start(enter) the game.
	gameStartButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gameStart, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game start button (%s)", gameStart)
	}
	defer gameStartButton.Release(ctx)
	if err := gameStartButton.FocusAndWait(ctx, timeout); err != nil {
		return errors.Wrapf(err, "failed to focus on the game start button (%s)", gameStart)
	}
	// Make sure the game is fully launched.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	id0 := ws[0].ID
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gameStartButton.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed to click the game start button (%s)", gameStart)
		}
		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			return errors.Wrapf(err, "failed to get the window")
		}
		// The window should turn into fullscreen mode when game starts.
		if w0.State != ash.WindowStateFullscreen {
			return errors.New("hasn't entered the game yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to start the game %s", name)
	}
	return nil
}

// PressKeyInGame presses a given key and waited for a given duration. Video games take time
// to process keyboard events so the intervals between two events are necessary.
func PressKeyInGame(ctx context.Context, kb *input.KeyboardEventWriter, s string, duration time.Duration) error {
	if err := kb.Accel(ctx, s); err != nil {
		return errors.Wrap(err, "failed to press the key")
	}
	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	return nil
}

// HoldKeyInGame holds a key for a given duration. Holding keys (especially arrow keys) is very common in
// video game playing.
func HoldKeyInGame(ctx context.Context, kb *input.KeyboardEventWriter, s string, duration time.Duration) error {
	if err := kb.AccelPress(ctx, s); err != nil {
		return errors.Wrap(err, "failed to long press the key")
	}
	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	if err := kb.AccelRelease(ctx, s); err != nil {
		return errors.Wrap(err, "failed to release the key")
	}
	return nil
}

// ExitGame holds esc key and exits the game.
func ExitGame(ctx context.Context, kb *input.KeyboardEventWriter, webpage *ui.Node) error {
	if err := HoldKeyInGame(ctx, kb, "esc", 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to hold the sec key")
	}
	exitButton, err := webpage.DescendantWithTimeout(ctx, ui.FindParams{Name: "Exit game", Role: ui.RoleTypeButton}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the exit button")
	}
	defer exitButton.Release(ctx)
	if err := exitButton.FocusAndWait(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to focus on the exit button")
	}
	if err := exitButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the exit button")
	}
	return nil
}

