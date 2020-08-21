// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type chromePIPEnergyAndPowerTestParams struct {
	bigPIP       bool
	layerOverPIP bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPEnergyAndPower,
		Desc:         "Measures energy and power usage of Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		Data:         []string{"bear-320x240.h264.mp4", "pip.html"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "small",
			Val:  chromePIPEnergyAndPowerTestParams{bigPIP: false, layerOverPIP: false},
		}, {
			Name: "big",
			Val:  chromePIPEnergyAndPowerTestParams{bigPIP: true, layerOverPIP: false},
		}, {
			Name: "small_blend",
			Val:  chromePIPEnergyAndPowerTestParams{bigPIP: false, layerOverPIP: true},
		}, {
			Name: "big_blend",
			Val:  chromePIPEnergyAndPowerTestParams{bigPIP: true, layerOverPIP: true},
		}},
	})
}

func ChromePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait for CPU to cool down: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	var pipButtonCenterString string
	if err := conn.Call(ctx, &pipButtonCenterString, "getPIPButtonCenter"); err != nil {
		s.Fatal("Failed to get center of PIP button: ", err)
	}

	var pipButtonCenterInWebContents coords.Point
	if n, err := fmt.Sscanf(pipButtonCenterString, "%v,%v", &pipButtonCenterInWebContents.X, &pipButtonCenterInWebContents.Y); err != nil {
		s.Fatalf("Failed to parse center of PIP button (successfully parsed %v of 2 tokens): %v", n, err)
	}

	webContentsView, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: "WebContentsViewAura"})
	if err != nil {
		s.Fatal("Failed to get web contents view: ", err)
	}
	defer webContentsView.Release(ctx)

	if err := mouse.Click(ctx, tconn, webContentsView.Location.TopLeft().Add(pipButtonCenterInWebContents), mouse.LeftButton); err != nil {
		s.Fatal("Failed to click PIP button: ", err)
	}

	pipWindowFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get PIP resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)

	if err := mouse.Move(ctx, tconn, resizeHandle.Location.CenterPoint(), time.Second); err != nil {
		s.Fatal("Failed to move mouse to PIP resize handle: ", err)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press left mouse button for dragging PIP resize handle: ", err)
	}
	params := s.Param().(chromePIPEnergyAndPowerTestParams)
	workAreaTopLeft := info.WorkArea.TopLeft()
	var resizeEnd coords.Point
	if params.bigPIP {
		resizeEnd = workAreaTopLeft
	} else {
		resizeEnd = info.WorkArea.BottomRight().Sub(coords.NewPoint(1, 1))
	}
	if err := mouse.Move(ctx, tconn, resizeEnd, time.Second); err != nil {
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			s.Fatal("Failed to move mouse for dragging PIP resize handle, and then failed to release left mouse button: ", err)
		}
		s.Fatal("Failed to move mouse for dragging PIP resize handle: ", err)
	}
	if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to release left mouse button for dragging PIP resize handle: ", err)
	}

	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location-change events to be propagated to the automation API: ", err)
	}

	pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP window: ", err)
	}
	defer pipWindow.Release(ctx)

	if params.bigPIP {
		maxWidth := info.WorkArea.Width / 2
		maxHeight := info.WorkArea.Height / 2
		// Expect the PIP window to have either the maximum width or the maximum
		// height, depending on how their ratio compares with 4x3.
		if maxWidth*3 <= maxHeight*4 {
			if pipWindow.Location.Width != maxWidth {
				s.Fatalf("PIP window is %v (after resize attempt). It should have width %d", pipWindow.Location.Size(), maxWidth)
			}
		} else {
			if pipWindow.Location.Height != maxHeight {
				s.Fatalf("PIP window is %v (after resize attempt). It should have height %d", pipWindow.Location.Size(), maxHeight)
			}
		}
	} else {
		// The minimum size of a Chrome PIP window is 260x146. The aspect ratio of the
		// video is 4x3, and so the minimum width 260 corresponds to a height of 195.
		if pipWindow.Location.Width != 260 || pipWindow.Location.Height != 195 {
			s.Fatalf("PIP window is %v. It should be (260 x 195)", pipWindow.Location.Size())
		}
	}

	if params.layerOverPIP {
		chromeIcon, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Google Chrome", ClassName: "ash/ShelfAppButton"})
		if err != nil {
			s.Fatal("Failed to get Chrome icon: ", err)
		}
		defer chromeIcon.Release(ctx)

		if err := mouse.Move(ctx, tconn, chromeIcon.Location.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse to Chrome icon: ", err)
		}
		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			s.Fatal("Failed to press left mouse button for dragging Chrome icon: ", err)
		}
		defer mouse.Release(ctx, tconn, mouse.LeftButton)
		if err := mouse.Move(ctx, tconn, pipWindow.Location.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse for dragging Chrome icon: ", err)
		}
	} else {
		// Ensure that the PIP window will show no controls or resize shadows.
		if err := mouse.Move(ctx, tconn, workAreaTopLeft.Add(coords.NewPoint(20, 20)), time.Second); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
	}

	extraConn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer extraConn.Close()

	// Wait for chrome://settings to be quiescent. We want data that we
	// could extrapolate, as in a steady state that could last for hours.
	if err := webutil.WaitForQuiescence(ctx, extraConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	const timelineDuration = time.Minute
	if err := testing.Sleep(ctx, timelineDuration); err != nil {
		s.Fatalf("Failed to wait %v: %v", timelineDuration, err)
	}
	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
