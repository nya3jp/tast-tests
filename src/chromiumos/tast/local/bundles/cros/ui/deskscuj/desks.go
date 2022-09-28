// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deskscuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/cuj/inputsimulations"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
)

// openDesk creates and initializes a desk based on the info in |desk|.
// |i| is the index of the new desk. If i is 0, the desk will just be
// initialized, because the first desk is created and activated by
// default. Each url in |urls| is opened in a separate window. Each
// successive call to openDesk must have an |i| value exactly 1 more than
// in the previous call, with the first call to this function expected to
// be 0.
func openDesk(ctx context.Context, tconn *chrome.TestConn, cs ash.ConnSource, urls []string, expectedNumWindows, i int) ([]cuj.TabConn, error) {
	if i != 0 {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			return nil, errors.Wrapf(err, "failed to create desk %d", i)
		}
		if err := ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			return nil, errors.Wrapf(err, "failed to activate desk %d", i)
		}
	}

	deskTabs, err := cuj.NewTabsByURLs(ctx, cs, true, urls)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open urls for desk %d", i)
	}

	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
	}); err != nil {
		return nil, errors.Wrap(err, "failed to set each window state to normal")
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if numWindows := len(ws); numWindows != expectedNumWindows {
		return nil, errors.Errorf("unexpected number of open windows after setting up desk %d: got %d windows, expected %d windows", i, numWindows, expectedNumWindows)
	}

	return deskTabs, nil
}

// setUpDesks creates 3 desks in addition to the initial default desk,
// and opens up a variety of windows on each desk. At the end of
// setUpDesks, there will be a total of 4 desks, with rightmost desk
// being active. Desk 1 has a separate window with extra tabs for
// additional RAM pressure. This function returns a list of actions
// to be performed on the corresponding desk, as well as the total
// number of windows that should be open after setUpDesks completes.
//
// Desks are arranged based on the following:
// Desk 1:
//   - Windows: 6
//   - User Input: Mouse Scroll Wheel
//
// Desk 2:
//   - Windows: 4
//   - User Input: Trackpad Scroll
//
// Desk 3:
//   - Windows: 2
//   - User Input: Mouse Movement
//
// Desk 4:
//   - Windows: 1
//   - User Input: Keyboard typing
func setUpDesks(ctx context.Context, tconn, bTconn *chrome.TestConn, cs ash.ConnSource, kw *input.KeyboardEventWriter, mw *input.MouseEventWriter, tpw *input.TrackpadEventWriter, tw *input.TouchEventWriter) ([]action.Action, int, error) {
	const notes = "The quick brown fox jumps over the lazy dog in the afternoon on Saturday!"

	docsURL, err := cuj.GetTestDocURL()
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get Google Doc URL")
	}

	// Open additional tabs for RAM pressure.
	tabs, err := cuj.NewTabs(ctx, cs, false, 3)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to open multiple tabs in a window")
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get the primary display info")
	}

	var totalOpenWindows int
	var onVisitActions []action.Action
	for i, desk := range []struct {
		urls               []string      // A list of urls to open for this desk.
		expectedNumWindows int           // Expected number of windows that should be open after desk setup.
		onVisitAction      action.Action // Unique user input action to perform on this desk.
	}{
		{
			urls: []string{
				"https://crosvideo.appspot.com/?codec=h264_60&loop=true&mute=true",
				"https://bugs.chromium.org/p/chromium/issues/list",
				"https://docs.google.com/document",
				"https://news.google.com/?hl=en-US&gl=US&ceid=US:en",
				"https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA",
			},
			onVisitAction: func(ctx context.Context) error {
				if err := inputsimulations.RunDragMouseCycle(ctx, tconn, info); err != nil {
					return err
				}

				return inputsimulations.ScrollMouseDownFor(ctx, mw, 500*time.Millisecond, 6*time.Second)
			},
			expectedNumWindows: 6, // This includes the 5 websites defined in urls, and the additional window for RAM pressure.
		},
		{
			urls: []string{
				"https://chrome.google.com/webstore/category/extensions",
				"https://mail.google.com",
				"https://www.nytimes.com/",
				docsURL,
			},
			onVisitAction: func(ctx context.Context) error {
				if err := inputsimulations.RunDragMouseCycle(ctx, tconn, info); err != nil {
					return err
				}

				if err := inputsimulations.ScrollDownFor(ctx, tpw, tw, time.Second, 6*time.Second); err != nil {
					return errors.Wrap(err, "failed to scroll down with trackpad")
				}
				return nil
			},
			expectedNumWindows: 4,
		},
		{
			urls: []string{
				"https://docs.google.com/document/d/19R_RWgGAqcHtgXic_YPQho7EwZyUAuUZyBq4n_V-BJ0/edit?usp=sharing",
				"https://webglsamples.org/aquarium/aquarium.html?numFish=1000",
			},
			onVisitAction: func(ctx context.Context) error {
				return inputsimulations.MoveMouseFor(ctx, tconn, 6*time.Second)
			},
			expectedNumWindows: 2,
		},
		{
			urls: []string{
				"https://docs.new/",
			},
			onVisitAction:      kw.TypeAction(notes),
			expectedNumWindows: 1,
		},
	} {
		totalOpenWindows += desk.expectedNumWindows
		deskTabs, err := openDesk(ctx, tconn, cs, desk.urls, totalOpenWindows, i)
		if err != nil {
			return nil, totalOpenWindows, errors.Wrapf(err, "failed to complete setup for desk %d", i)
		}

		onVisitActions = append(onVisitActions, desk.onVisitAction)
		tabs = append(tabs, deskTabs...)
	}

	// Close connections to each tab because we don't need them.
	for _, tab := range tabs {
		if err := tab.Conn.Close(); err != nil {
			return nil, totalOpenWindows, errors.Wrapf(err, "failed to close connection to %s", tab.URL)
		}
	}

	return onVisitActions, totalOpenWindows, nil
}
