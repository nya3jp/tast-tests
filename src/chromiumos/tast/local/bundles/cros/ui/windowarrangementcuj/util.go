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

// TestConnection holds things that facilitate interaction with the DUT.
type TestConnection struct {
	Cr              *chrome.Chrome
	Cs              ash.ConnSource
	Tconn           *chrome.TestConn
	Cleanup         func(ctx context.Context) error
	CloseAboutBlank func(ctx context.Context) error
	BTconn          *chrome.TestConn
}

// DragPoints holds three points, to signify a drag from the first point
// to the second point, then the third point, and back to the first point.
type DragPoints [3]coords.Point

// SetupChrome creates ash-chrome or lacros-chrome based on test parameters.
func SetupChrome(ctx, closeCtx context.Context, s *testing.State) (*TestConnection, error) {
	testParam := s.Param().(TestParam)

	connection := &TestConnection{
		nil,
		nil,
		nil,
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		nil,
	}
	var l *lacros.Lacros

	ok := false
	defer func() {
		if !ok {
			if err := connection.Cleanup(closeCtx); err != nil {
				s.Error("Failed to clean up after detecting error condition: ", err)
			}
		}
	}()

	if testParam.BrowserType == browser.TypeAsh {
		if testParam.Tablet {
			var err error
			if connection.Cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
				return nil, errors.Wrap(err, "failed to init chrome")
			}
			connection.Cleanup = connection.Cr.Close
		} else {
			connection.Cr = s.FixtValue().(*chrome.Chrome)
		}
		connection.Cs = connection.Cr

		var err error
		connection.BTconn, err = connection.Cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get TestAPIConn")
		}
	} else {
		var err error
		connection.Cr, l, connection.Cs, err = lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
		if err != nil {
			return nil, errors.Wrap(err, "failed to setup lacros")
		}
		connection.Cleanup = func(ctx context.Context) error {
			lacros.CloseLacros(ctx, l)
			return nil
		}

		if connection.BTconn, err = l.TestAPIConn(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to get lacros TestAPIConn")
		}
	}

	var err error
	connection.Tconn, err = connection.Cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test api")
	}

	if testParam.BrowserType == browser.TypeLacros {
		connection.CloseAboutBlank = func(ctx context.Context) error {
			return l.CloseAboutBlank(ctx, connection.Tconn, 0)
		}
	}

	ok = true
	return connection, nil
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
