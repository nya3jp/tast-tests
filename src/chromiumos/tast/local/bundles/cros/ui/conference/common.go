// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// WaitUIByName waits the specified DOM element by name.
func WaitUIByName(ctx context.Context, tconn *chrome.TestConn, name string, timeout time.Duration) error {
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: name}, timeout); err != nil {
		return errors.Wrapf(err, `failed to find %q `, name)
	}
	return nil
}

// ClickUIByName clicks the specified DOM element by name.
func ClickUIByName(ctx context.Context, tconn *chrome.TestConn, name string, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		node, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: name}, time.Second)
		if err != nil {
			return err
		}

		if err := testing.Sleep(ctx, time.Millisecond*500); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		defer node.Release(ctx)
		return node.LeftClick(ctx)
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, `failed to click %q button`, name)
	}
	return nil
}

func mouseMoveToCenter(ctx context.Context, tconn *chrome.TestConn) (int, int, error) {
	const xRatio, yRatio = .55, .55
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return 0, 0, err
	}

	dw := info.WorkArea.Width
	dh := info.WorkArea.Height
	mw, mh := int(float64(dw)*xRatio), int(float64(dh)*yRatio)

	if err := mouse.Move(ctx, tconn, coords.Point{X: mw, Y: mh}, time.Second); err != nil {
		return 0, 0, err
	}
	return mw, mh, nil
}

// EditSlide edits the specified slide after presenting.
func EditSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	mw, mh, err := mouseMoveToCenter(ctx, tconn)
	if err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		mouse.Click(ctx, tconn, coords.Point{X: mw, Y: mh}, mouse.LeftButton)
		testing.Sleep(ctx, time.Millisecond*50)
	}

	if err := kb.Type(ctx, "This is CUJ Testing"); err != nil {
		return err
	}

	if err := kb.Accel(ctx, "Esc"); err != nil {
		return errors.Wrap(err, "failed to press esc")
	}
	testing.Sleep(ctx, time.Millisecond*500)
	if err := kb.Accel(ctx, "Esc"); err != nil {
		return errors.Wrap(err, "failed to press esc")
	}

	return nil
}
