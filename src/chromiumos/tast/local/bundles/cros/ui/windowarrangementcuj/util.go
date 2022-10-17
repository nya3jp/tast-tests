// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package windowarrangementcuj contains helper util and test code for
// WindowArrangementCUJ.
package windowarrangementcuj

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// TestParam holds parameters of window arrangement cuj test variations.
type TestParam struct {
	BrowserType browser.Type
	Tablet      bool
}

// Connections holds things that facilitate interaction with the DUT.
type Connections struct {
	// Chrome interacts with the currently-running Chrome instance via
	// the Chrome DevTools protocol:
	// https://chromedevtools.github.io/devtools-protocol/
	Chrome *chrome.Chrome

	// Source is used to create new chrome.Conn connections.
	Source ash.ConnSource

	// TestConn is a connection to ash chrome.
	TestConn *chrome.TestConn

	// Cleanup resets everything to a clean state. It only needs to be
	// called if SetupChrome succeeds.
	Cleanup func(ctx context.Context) error

	// CloseBlankTab closes the blank tab that is created when lacros
	// is started.
	CloseBlankTab func(ctx context.Context) error

	// BrowserTestConn is a connection to ash chrome or lacros chrome,
	// depending on the browser in use.
	BrowserTestConn *chrome.TestConn

	// BrowserType is the browser type.
	BrowserType browser.Type

	// PipVideoTestURL is the URL of the PIP video test page.
	PipVideoTestURL string
}

// SetupChrome creates ash-chrome or lacros-chrome based on test parameters.
func SetupChrome(ctx, closeCtx context.Context, s *testing.State) (*Connections, error) {
	testParam := s.Param().(TestParam)

	var cleanupActionsInReverseOrder []action.Action

	connection := &Connections{
		Cleanup: func(ctx context.Context) error {
			var firstErr error
			for i := len(cleanupActionsInReverseOrder) - 1; i >= 0; i-- {
				if err := cleanupActionsInReverseOrder[i](ctx); firstErr == nil {
					firstErr = err
				}
			}
			return firstErr
		},
		CloseBlankTab: func(ctx context.Context) error { return nil },
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

	connection.BrowserType = testParam.BrowserType
	if testParam.BrowserType == browser.TypeAsh {
		connection.Chrome = s.FixtValue().(chrome.HasChrome).Chrome()
		connection.Source = connection.Chrome

		var err error
		connection.BrowserTestConn, err = connection.Chrome.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get TestAPIConn")
		}
	} else {
		var err error
		connection.Chrome, l, connection.Source, err = lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
		if err != nil {
			return nil, errors.Wrap(err, "failed to setup lacros")
		}
		cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
			lacros.CloseLacros(ctx, l)
			return nil
		})

		if connection.BrowserTestConn, err = l.TestAPIConn(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to get lacros TestAPIConn")
		}
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		srv.Close()
		return nil
	})
	connection.PipVideoTestURL = srv.URL + "/pip.html"

	var err error
	connection.TestConn, err = connection.Chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test api")
	}

	if testParam.BrowserType == browser.TypeLacros {
		connection.CloseBlankTab = func(ctx context.Context) error {
			return l.Browser().CloseWithURL(ctx, chrome.NewTabURL)
		}
	}

	ok = true
	return connection, nil
}

// cleanUp is used to execute a given cleanup action and report
// the resulting error if it is not nil. The intended usage is:
//
//	func Example(ctx, closeCtx context.Context) (retErr error) {
//	  ...
//	  defer cleanUp(closeCtx, action.Named("description of cleanup action", cleanup), &retErr)
//	  ...
//	}
func cleanUp(ctx context.Context, cleanup action.Action, retErr *error) {
	if err := cleanup(ctx); err != nil {
		if *retErr == nil {
			*retErr = err
		} else {
			testing.ContextLog(ctx, "Cleanup failed: ", err)
			testing.ContextLog(ctx, "Note: This cleanup failure is not the first error. The first error will be reported after all cleanup actions have been attempted")
		}
	}
}

// combineTabs is used to merge two browser windows, each consisting
// of a single tab, into one browser window with two tabs.
func combineTabs(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, duration time.Duration) (retErr error) {
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to ensure clamshell mode")
	}
	defer func() {
		if err := cleanup(ctx); retErr == nil && err != nil {
			retErr = errors.Wrap(err, "failed to clean up after ensuring clamshell mode")
		}
	}()

	tab := nodewith.Role(role.Tab).HasClass("Tab")
	tabPIP := tab.NameContaining("/pip.html - ")
	tabNoPIP := tab.NameRegex(regexp.MustCompile("/pip.html$"))

	firstTabRect, err := ui.Location(ctx, tabNoPIP)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the first tab")
	}
	secondTabRect, err := ui.Location(ctx, tabPIP)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the second tab")
	}

	if err := pc.Drag(
		firstTabRect.CenterPoint(),
		pc.DragTo(firstTabRect.BottomCenter(), duration),
		pc.DragTo(secondTabRect.CenterPoint(), duration),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag one browser tab to the other")
	}

	ws, err := getAllWindowsWorkingAroundB252552657(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	if len(ws) != 1 {
		return errors.Errorf("unexpected number of windows after trying to merge: got %d; expected 1", len(ws))
	}

	return nil
}

// removeExtraDesk removes the active desk and then ensures that a window that
// was on this removed desk is active. This window activation is important for
// the drag in combineTabs, because the tab to be dragged needs to be on top.
func removeExtraDesk(ctx context.Context, tconn *chrome.TestConn) error {
	w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.OnActiveDesk })
	if err != nil {
		return errors.Wrap(err, "failed to find window on active desk")
	}
	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to remove desk")
	}
	if err := w.ActivateWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to ensure suitable window activation")
	}
	return nil
}

// dragAndRestore performs a drag beginning at the first given point, proceeding
// through the others in order, and ending back at the first given point. Before
// ending the drag, dragAndRestore waits until every window has the same bounds
// as before the drag (as expected because the drag is a closed loop).
func dragAndRestore(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, duration time.Duration, p ...coords.Point) error {
	if len(p) < 2 {
		return errors.Errorf("expected at least two drag points, got %v", p)
	}

	wsInitial, err := getAllWindowsWorkingAroundB252552657(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get windows")
	}

	verifyBounds := func(ctx context.Context) error {
		for _, wInitial := range wsInitial {
			wNow, err := ash.GetWindow(ctx, tconn, wInitial.ID)
			if err != nil {
				return errors.Wrapf(err, "failed to look up %q window by ID %d (the app probably crashed)", wInitial.Title, wInitial.ID)
			}
			if !wNow.BoundsInRoot.Equals(wInitial.BoundsInRoot) {
				return errors.Errorf("%q window bounds not restored; changed from %v to %v", wNow.Title, wInitial.BoundsInRoot, wNow.BoundsInRoot)
			}
		}
		return nil
	}
	verifyBoundsTimeout := &testing.PollOptions{Timeout: 10 * time.Second}

	var dragSteps []uiauto.Action
	for i := 1; i < len(p); i++ {
		dragSteps = append(dragSteps, pc.DragTo(p[i], duration))
	}
	dragSteps = append(dragSteps, pc.DragTo(p[0], duration), func(ctx context.Context) error {
		if err := testing.Poll(ctx, verifyBounds, verifyBoundsTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for expected window bounds before ending drag")
		}
		return nil
	})
	if err := pc.Drag(p[0], dragSteps...)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag")
	}

	return nil
}

// getAllWindowsWorkingAroundB252552657 calls ash.GetAllWindows and filters out
// PIP windows because they are not supposed to be returned by ash.GetAllWindows
// in the first place (see b/252552657#comment7).
// TODO(b/252552657): When the bug is fixed, remove this and update callers to
// use ash.GetAllWindows directly.
func getAllWindowsWorkingAroundB252552657(ctx context.Context, tconn *chrome.TestConn) ([]*ash.Window, error) {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return nil, err
	}

	var filtered []*ash.Window
	for _, w := range ws {
		if w.State != ash.WindowStatePIP {
			filtered = append(filtered, w)
		}
	}
	return filtered, nil
}
