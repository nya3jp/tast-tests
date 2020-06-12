// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebYoutubePIPEnergyAndPower,
		Desc:         "Measures energy and power consumption of a YouTube video playing in Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Vars: []string{
			"ui.WebYoutubePIPEnergyAndPower.username",
			"ui.WebYoutubePIPEnergyAndPower.password",
			"ui.WebYoutubePIPEnergyAndPower.ytExperiments",
		},
		Params: []testing.Param{{
			Name: "clamshell",
			Val:  false,
		}, {
			Name:              "tablet",
			Val:               true,
			ExtraSoftwareDeps: []string{"tablet_mode"},
		}},
	})
}

func WebYoutubePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	// Do these before anything else that may cause an error. If one
	// of these causes an error, that is the error we should report.
	username := s.RequiredVar("ui.WebYoutubePIPEnergyAndPower.username")
	password := s.RequiredVar("ui.WebYoutubePIPEnergyAndPower.password")
	ytExperiments := s.RequiredVar("ui.WebYoutubePIPEnergyAndPower.ytExperiments")

	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(username, password, "gaia-id"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer audio.Unmute(ctx)

	tabletMode := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(ctx)

	var pointerController pointer.Controller
	var tew *input.TouchscreenEventWriter
	var stw *input.SingleTouchEventWriter
	if tabletMode {
		touchController, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create touch controller: ", err)
		}
		pointerController = touchController
		tew = touchController.Touchscreen()
		stw = touchController.EventWriter()
	} else {
		pointerController = pointer.NewMouseController(tconn)
	}
	defer pointerController.Close()

	ytConn, err := cr.NewConn(ctx, "https://www.youtube.com/watch?v=EEIk7gwjgIM&absolute_experiments="+ytExperiments, cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to navigate to YouTube web page: ", err)
	}
	defer ytConn.Close()

	if err := webutil.WaitForQuiescence(ctx, ytConn, time.Minute); err != nil {
		s.Fatal("Failed to wait for YouTube web page to load: ", err)
	}

	var videoCenterString string
	if err := ytConn.Call(ctx, &videoCenterString, `
		function() {
			const bounds = document.querySelector('video').getBoundingClientRect();
			const left = Math.max(0, Math.min(bounds.x, bounds.x + bounds.width));
			const right = Math.min(window.innerWidth, Math.max(bounds.x, bounds.x + bounds.width));
			const top = Math.max(0, Math.min(bounds.y, bounds.y + bounds.height));
			const bottom = Math.min(window.innerHeight, Math.max(bounds.y, bounds.y + bounds.height));
			return Math.round(left + 0.5 * (right - left)) + ',' + Math.round(top + 0.5 * (bottom - top));
		}`); err != nil {
		s.Fatal("Failed to get center of video: ", err)
	}

	var videoCenterInWebContents coords.Point
	if n, err := fmt.Sscanf(videoCenterString, "%v,%v", &videoCenterInWebContents.X, &videoCenterInWebContents.Y); err != nil {
		s.Fatalf("Failed to parse center of video (successfully parsed %v of 2 tokens): %v", n, err)
	}

	webContentsView, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: "WebContentsViewAura"})
	if err != nil {
		s.Fatal("Failed to get web contents view: ", err)
	}
	defer webContentsView.Release(ctx)

	// TODO(https://crbug.com/1093933): Simulate a real workflow in tablet mode.
	// Mouse input in tablet mode is unrealistic, and so we should use touch
	// gestures. The problem is https://crbug.com/1095176.
	videoCenter := webContentsView.Location.TopLeft().Add(videoCenterInWebContents)
	if err := mouse.Click(ctx, tconn, videoCenter, mouse.RightButton); err != nil {
		s.Fatal("Failed to right click center of video for app context menu: ", err)
	}
	if err := mouse.Click(ctx, tconn, videoCenter, mouse.RightButton); err != nil {
		s.Fatal("Failed to right click center of video for system context menu: ", err)
	}

	pipMenuItemFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "MenuItemView"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipMenuItemFindParams, 15*time.Second); err != nil {
		s.Fatal("Failed to wait for menu with PIP option: ", err)
	}

	pipMenuItem, err := chromeui.Find(ctx, tconn, pipMenuItemFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP option on menu: ", err)
	}
	defer pipMenuItem.Release(ctx)

	if err := pointerController.Press(ctx, pipMenuItem.Location.CenterPoint()); err != nil {
		s.Fatal("Failed to press mouse/touch on PIP option on menu: ", err)
	}
	if err := pointerController.Release(ctx); err != nil {
		s.Fatal("Failed to release mouse/touch on PIP option on menu: ", err)
	}

	pipWindowFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, time.Minute); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if tabletMode {
		if err := ash.DragToShowHomescreen(ctx, tew.Width(), tew.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to drag to show home launcher: ", err)
		}
		// Tap the PIP window in preparation for the resizing swipe. Otherwise, that
		// swipe will move the PIP window instead of resizing it.
		pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
		if err != nil {
			s.Fatal("Failed to get PIP window: ", err)
		}
		defer pipWindow.Release(ctx)
		if err := pointerController.Press(ctx, pipWindow.Location.CenterPoint()); err != nil {
			s.Fatal("Failed to press touch on center of PIP window: ", err)
		}
		if err := pointerController.Release(ctx); err != nil {
			s.Fatal("Failed to release touch on center of PIP window: ", err)
		}
	} else {
		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get windows: ", err)
		}
		if windowsCount := len(windows); windowsCount != 1 {
			s.Fatal("Expected 1 window; found ", windowsCount)
		}
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventMinimize); err != nil {
			s.Fatal("Failed to minimize browser window: ", err)
		}
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)

	if err := pointerController.Press(ctx, resizeHandle.Location.CenterPoint()); err != nil {
		s.Fatal("Failed to press mouse/touch on PIP resize handle: ", err)
	}
	if err := pointerController.Move(ctx, resizeHandle.Location.CenterPoint(), coords.Point{X: 0, Y: 0}, time.Second); err != nil {
		s.Fatal("Failed to move mouse/touch dragging PIP resize handle: ", err)
	}
	if err := pointerController.Release(ctx); err != nil {
		s.Fatal("Failed to release mouse/touch on PIP resize handle: ", err)
	}

	energyMetrics := power.NewRAPLMetrics()
	if err := energyMetrics.Setup(ctx, "web_youtube_pip_energy_"); err != nil {
		s.Fatal("Failed to set up energy metrics: ", err)
	}

	powerMetrics := power.NewRAPLPowerMetrics()
	if err := powerMetrics.Setup(ctx, "web_youtube_pip_power_"); err != nil {
		s.Fatal("Failed to set up power metrics: ", err)
	}

	pv := perf.NewValues()
	if err := energyMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start collecting energy metrics: ", err)
	}
	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start collecting power metrics: ", err)
	}
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait a minute: ", err)
	}
	if err := energyMetrics.Snapshot(ctx, pv); err != nil {
		s.Fatal("Failed to collect energy metrics: ", err)
	}
	if err := powerMetrics.Snapshot(ctx, pv); err != nil {
		s.Fatal("Failed to collect power metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
