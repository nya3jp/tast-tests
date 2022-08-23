// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/input"
)

// deskInfo contains information needed to initialize a single
// desk.
type deskInfo struct {
	urls               []string      // A list of urls to open for this desk.
	expectedNumWindows int           // Expected number of windows that should be open after desk setup.
	onVisitAction      action.Action // Unique user input action to perform on this desk.
}

// openDesk opens and initializes a desk based on the info in |desk|.
// |i| is the desk index of the new desk. Each url in |desk.url| is
// opened in a separate window.
func openDesk(ctx context.Context, tconn *chrome.TestConn, cs ash.ConnSource, desk deskInfo, expectedNumWindows, i int) ([]cuj.TabConn, error) {
	if i != 0 {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			return nil, errors.Wrapf(err, "failed to create desk %d", i)
		}
		if err := ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			return nil, errors.Wrapf(err, "failed to activate desk %d ", i)
		}
	}

	deskTabs, err := cuj.NewTabsByURLs(ctx, cs, true, desk.urls)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open urls for desk %d", i)
	}

	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
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

// setUpDesks creates 3 additional desks, and opens up a variety of
// windows on each desk. Desk 1 also has a separate window with
// extra tabs for additional RAM pressure. Desks are arranged
// according to the following:
// Desk 1:
//   - Windows: 8
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
func setUpDesks(ctx context.Context, tconn, bTconn *chrome.TestConn, cs ash.ConnSource, kw *input.KeyboardEventWriter, mw *input.MouseEventWriter, tpw *input.TrackpadEventWriter, tw *input.TouchEventWriter) ([]deskInfo, error) {
	const notes = "The quick brown fox jumps over the lazy dog in the afternoon on Saturday!"

	docsURL, err := cuj.GetTestDocURL()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Google Doc URL")
	}

	desks := []deskInfo{
		deskInfo{
			urls: []string{
				"https://crosvideo.appspot.com/?codec=h264_60&loop=true&mute=true",
				"https://webglsamples.org/aquarium/aquarium.html?numFish=1000",
				"https://bugs.chromium.org/p/chromium/issues/list",
				"https://docs.google.com/document",
				"https://chrome.google.com/webstore/category/extensions",
				"https://news.google.com/?hl=en-US&gl=US&ceid=US:en",
				docsURL,
			},
			onVisitAction: func(ctx context.Context) error {
				return inputsimulations.ScrollMouseDownFor(ctx, mw, 500*time.Millisecond, 6*time.Second)
			},
			expectedNumWindows: 8, // This includes the 7 websites defined in urls, and the additional window for RAM pressure.
		},
		deskInfo{
			urls: []string{
				"https://bugs.chromium.org/p/chromium/issues/list",
				"https://mail.google.com",
				"https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA",
				"https://docs.google.com/spreadsheets/d/1I9jmmdWkBaH6Bdltc2j5KVSyrJYNAhwBqMmvTdmVOgM/edit?usp=sharing&resourcekey=0-60wBsoTfOkoQ6t4yx2w7FQ",
			},
			onVisitAction: func(ctx context.Context) error {
				if err := inputsimulations.ScrollDownFor(ctx, tpw, tw, time.Second, 6*time.Second); err != nil {
					return errors.Wrap(err, "failed to scroll down with trackpad")
				}
				return nil
			},
			expectedNumWindows: 4,
		},
		deskInfo{
			urls: []string{
				"https://crosvideo.appspot.com/?codec=h264_60&loop=true&mute=true",
				"https://webglsamples.org/aquarium/aquarium.html?numFish=1000",
			},
			onVisitAction: func(ctx context.Context) error {
				return inputsimulations.MoveMouseFor(ctx, tconn, 6*time.Second)
			},
			expectedNumWindows: 2,
		},
		deskInfo{
			urls: []string{
				"https://docs.new/",
			},
			onVisitAction:      kw.TypeAction(notes),
			expectedNumWindows: 1,
		},
	}

	// Open additional tabs for RAM pressure.
	tabs, err := cuj.NewTabs(ctx, cs, false, 3)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open multiple tabs in a window")
	}

	var totalOpenWindows int
	for i, desk := range desks {
		totalOpenWindows += desk.expectedNumWindows
		deskTabs, err := openDesk(ctx, tconn, cs, desk, totalOpenWindows, i)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to complete setup for desk %d", i)
		}
		tabs = append(tabs, deskTabs...)
	}

	// Close connections to each tab because we don't need them.
	for _, tab := range tabs {
		if err := tab.Conn.Close(); err != nil {
			return nil, errors.Wrapf(err, "failed to close connection to %s", tab.URL)
		}
	}

	return desks, nil
}
