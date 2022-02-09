// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package windowarrangementcuj contains helper util and test code for
// WindowArrangementCUJ.
package windowarrangementcuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// TestParam holds parameters of window arrangement cuj test variations.
type TestParam struct {
	BrowserType browser.Type
	Tablet      bool
	Tracing     bool
	Validation  bool
}

// ChromeCleanUpFunc defines the clean up function of chrome browser.
type ChromeCleanUpFunc func(ctx context.Context) error

// CloseAboutBlankFunc defines the helper to close about:blank page. It is
// implemented for lacros-chrome and no-op for ash-chrome.
type CloseAboutBlankFunc func(ctx context.Context) error

// DragPoints holds three points, to signify a drag from the first point
// to the second point, then the third point, and back to the first point.
type DragPoints [3]coords.Point

// SetupChrome creates ash-chrome or lacros-chrome based on test parameters.
func SetupChrome(ctx, closeCtx context.Context, s *testing.State) (*chrome.Chrome, ash.ConnSource, *chrome.TestConn, ChromeCleanUpFunc, CloseAboutBlankFunc, *chrome.TestConn, error) {
	testParam := s.Param().(TestParam)

	var cr *chrome.Chrome
	var cs ash.ConnSource
	var l *lacros.Lacros
	var bTconn *chrome.TestConn

	cleanup := func(ctx context.Context) error { return nil }
	closeAboutBlank := func(ctx context.Context) error { return nil }

	ok := false
	defer func() {
		if !ok {
			if err := cleanup(closeCtx); err != nil {
				s.Error("Failed to clean up after detecting error condition: ", err)
			}
		}
	}()

	if testParam.BrowserType == browser.TypeAsh {
		if testParam.Tablet {
			var err error
			if cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
				return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to init chrome")
			}
			cleanup = cr.Close
		} else {
			cr = s.FixtValue().(*chrome.Chrome)
		}
		cs = cr

		var err error
		bTconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to get TestAPIConn")
		}
	} else {
		var err error
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), testParam.BrowserType)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to setup lacros")
		}
		cleanup = func(ctx context.Context) error {
			lacros.CloseLacros(ctx, l)
			return nil
		}

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to get lacros TestAPIConn")
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to conect to test api")
	}

	if testParam.BrowserType == browser.TypeLacros {
		closeAboutBlank = func(ctx context.Context) error {
			return l.CloseAboutBlank(ctx, tconn, 0)
		}
	}

	ok = true
	return cr, cs, tconn, cleanup, closeAboutBlank, bTconn, nil
}

// Drag does the specified drag based on the documentation of DragPoints.
func Drag(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, p DragPoints, duration time.Duration) error {
	initialBoundsMap := make(map[int]coords.Rect)
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		initialBoundsMap[w.ID] = w.BoundsInRoot
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to get bounds of all windows")
	}

	if err := pc.Drag(
		p[0],
		pc.DragTo(p[1], duration),
		pc.DragTo(p[2], duration),
		pc.DragTo(p[0], duration),
		func(ctx context.Context) error {
			// When you are moving/resizing a window, its bounds can take a moment
			// to update after the pointer moves. We need to wait for the expected
			// final window bounds before ending the drag. The final bounds should
			// match the initial bounds because the drag begins and ends at p[0].
			// If the drag does not move/resize any windows, this code is okay as
			// the final bounds should match the initial bounds in that case too.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
					if initialBounds := initialBoundsMap[w.ID]; !w.BoundsInRoot.Equals(initialBounds) {
						return errors.Errorf("got window bounds %v; want %v", w.BoundsInRoot, initialBounds)
					}
					return nil
				}); err != nil {
					return errors.Wrap(err, "failed to verify bounds of all windows")
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to wait for window bounds to be what they were before the drag")
			}
			return nil
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag")
	}

	return nil
}
