// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskswitchcuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
)

// simpleWebsites are websites to be opened in individual browsers
// with no additional setup required.
// 1. WebGL Aquarium -- considerable load on graphics.
// 2. Chromium issue tracker -- considerable amount of elements.
// 3. CrosVideo -- customizable video player.
var simpleWebsites = []string{
	"https://bugs.chromium.org/p/chromium/issues/list",
	"https://crosvideo.appspot.com/?codec=h264_60&loop=true&mute=true",
	"https://webglsamples.org/aquarium/aquarium.html?numFish=1000",
}

// openChromeTabs opens Chrome tabs and returns the number of windows
// that were opened.
//
// This function opens an individual window for each URL in
// simpleWebsites. It also opens a window with multiple tabs, to
// increase RAM pressure during the test.
func openChromeTabs(ctx context.Context, tconn, bTconn *chrome.TestConn, cs ash.ConnSource, bt browser.Type, tabletMode bool) (int, error) {
	const numExtraWebsites = 5

	// Keep track of the initial number of windows, to ensure
	// we open the right number of windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get window list")
	}
	initialNumWindows := len(ws)

	// Open up a single window with a lot of tabs, to increase RAM pressure.
	tabs, err := cuj.NewTabs(ctx, cs, false, numExtraWebsites)
	if err != nil {
		return 0, errors.Wrap(err, "failed to bulk open tabs")
	}

	// Lacros specific setup to close "New Tab" window.
	if bt == browser.TypeLacros {
		// Don't include the "New Tab" window in the initial window count.
		initialNumWindows--

		if err := browser.CloseTabByTitle(ctx, bTconn, "New Tab"); err != nil {
			return 0, errors.Wrap(err, `failed to close "New Tab" tab`)
		}
	}

	// Also open a large slide deck for RAM pressure.
	slidesURL, err := cuj.GetDriveURL(cuj.DriveTypeSlides)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get Google Slides URL")
	}
	simpleWebsites := append(simpleWebsites, slidesURL)

	// Open up individual window for each website in simpleWebsites.
	taskSwitchTabs, err := cuj.NewTabsByURLs(ctx, cs, true, simpleWebsites)
	if err != nil {
		return 0, err
	}
	tabs = append(tabs, taskSwitchTabs...)

	// Close all current connections to tabs, because we don't need them.
	for _, t := range tabs {
		if err := t.Conn.Close(); err != nil {
			return 0, errors.Wrapf(err, "failed to close connection to %s", t.URL)
		}
	}

	if !tabletMode {
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
		}); err != nil {
			return 0, errors.Wrap(err, "failed to set each window to normal state")
		}
	}

	// Expected number of browser windows should include the number
	// of websites in |simpleWebsites|, and the window with many tabs.
	expectedNumBrowserWindows := len(simpleWebsites) + 1
	if ws, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return 0, errors.Wrap(err, "failed to get window list after opening Chrome tabs")
	} else if expectedNumWindows := expectedNumBrowserWindows + initialNumWindows; len(ws) != expectedNumWindows {
		return 0, errors.Wrapf(err, "unexpected number of windows open after launching Chrome tabs, got: %d, expected: %d", len(ws), expectedNumWindows)
	}

	return expectedNumBrowserWindows, nil
}
