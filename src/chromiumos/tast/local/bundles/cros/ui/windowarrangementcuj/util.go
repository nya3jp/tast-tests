// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package windowarrangementcuj contains helper util and test code for
// WindowArrangementCUJ.
package windowarrangementcuj

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
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

const pkgName = "org.chromium.arc.testapp.pictureinpicturevideo"

// TestParam holds parameters of window arrangement cuj test variations.
type TestParam struct {
	BrowserType browser.Type
	Tablet      bool
	Tracing     bool
	Validation  bool
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

	// PipVideoTestURL is the URL of the PIP video test page.
	PipVideoTestURL string

	// ARC holds resources related to the ARC session.
	ARC *arc.ARC

	// ARCUI interacts with the ARC UI Automator server.
	ARCUI *ui.Device

	// ArcVideoActivity is an ARC activity that plays a video, looped.
	// If you minimize it, it plays the video in PIP.
	ArcVideoActivity *arc.Activity

	// WithTestVideo provides the test video URI to ArcVideoActivity.
	WithTestVideo arc.ActivityStartOption
}

// DragPoints holds three points, to signify a drag from the first point
// to the second point, then the third point, and back to the first point.
type DragPoints [3]coords.Point

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

	if testParam.BrowserType == browser.TypeAsh {
		if testParam.Tablet {
			var err error
			if connection.Chrome, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
				return nil, errors.Wrap(err, "failed to init chrome")
			}
			cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, connection.Chrome.Close)
			if connection.ARC, err = arc.New(ctx, s.OutDir()); err != nil {
				return nil, errors.Wrap(err, "failed to init ARC")
			}
		} else {
			connection.Chrome = s.FixtValue().(*arc.PreData).Chrome
			connection.ARC = s.FixtValue().(*arc.PreData).ARC
		}
		connection.Source = connection.Chrome

		var err error
		connection.BrowserTestConn, err = connection.Chrome.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get TestAPIConn")
		}
	} else {
		var err error
		connection.Chrome, l, connection.Source, err = lacros.Setup(ctx, s.FixtValue().(*arc.PreData).Chrome, browser.TypeLacros)
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

		connection.ARC = s.FixtValue().(*arc.PreData).ARC
	}

	var err error
	connection.ARCUI, err = connection.ARC.NewUIDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize ARC UI automator")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, connection.ARCUI.Close)

	if err := connection.ARC.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		return nil, errors.Wrap(err, "failed to install ARC app")
	}
	connection.ArcVideoActivity, err = arc.NewActivity(connection.ARC, pkgName, ".VideoActivity")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ARC activity")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		connection.ArcVideoActivity.Close()
		return nil
	})

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		srv.Close()
		return nil
	})
	connection.PipVideoTestURL = srv.URL + "/pip.html"

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse test server URL")
	}
	hostPort, err := strconv.Atoi(srvURL.Port())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse test server port")
	}
	androidPort, err := connection.ARC.ReverseTCP(ctx, hostPort)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start reverse port forwarding")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		return connection.ARC.RemoveReverseTCP(ctx, androidPort)
	})
	connection.WithTestVideo = arc.WithExtraString("video_uri", fmt.Sprintf("http://localhost:%d/shaka_720.webm", androidPort))

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
// func Example(ctx, closeCtx context.Context) (retErr error) {
//   ...
//   defer cleanUp(closeCtx, action.Named("description of cleanup action", cleanup), &retErr)
//   ...
// }
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

// CombineTabs is used to merge two browser windows, each consisting
// of a single tab, into one browser window with two tabs.
func CombineTabs(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, duration time.Duration) (retErr error) {
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
	firstTabRect, err := ui.Location(ctx, tab.First())
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the first tab")
	}
	secondTabRect, err := ui.Location(ctx, tab.Nth(1))
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

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	if len(ws) != 1 {
		return errors.Errorf("unexpected number of windows after trying to merge: got %d; expected 1", len(ws))
	}

	return nil
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

	verifyBounds := func(ctx context.Context) error {
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			if initialBounds := initialBoundsMap[w.ID]; !w.BoundsInRoot.Equals(initialBounds) {
				return errors.Errorf("got window bounds %v; want %v", w.BoundsInRoot, initialBounds)
			}
			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to verify bounds of all windows")
		}
		return nil
	}
	verifyBoundsTimeout := &testing.PollOptions{Timeout: 10 * time.Second}

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
			if err := testing.Poll(ctx, verifyBounds, verifyBoundsTimeout); err != nil {
				testing.ContextLog(ctx, "Warning: Failed to wait for expected window bounds before ending drag (see https://crbug.com/1297297): ", err)
			}
			return nil
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag")
	}

	if err := testing.Poll(ctx, verifyBounds, verifyBoundsTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for expected window bounds after ending drag (which should never happen, regardless of https://crbug.com/1297297)")
	}

	return nil
}
