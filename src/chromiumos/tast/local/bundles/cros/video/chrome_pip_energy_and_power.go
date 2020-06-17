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
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type chromePIPEnergyAndPowerTestParams struct {
	tabletMode    bool
	bigPIP        bool
	videoFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPEnergyAndPower,
		Desc:         "Measures energy and power usage of Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"pip.html"},
		Timeout:      5 * time.Minute,
		Pre:          pre.ChromeVideo(),
		Params: []testing.Param{{
			Name:              "clamshell_small_with_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "clamshell_small_without_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
		}, {
			Name:              "tablet_small_with_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "tablet_mode"},
		}, {
			Name:              "tablet_small_without_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2, "tablet_mode"},
		}, {
			Name:              "clamshell_big_with_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "clamshell_big_without_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
		}, {
			Name:              "tablet_big_with_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "tablet_mode"},
		}, {
			Name:              "tablet_big_without_overlay",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2, "tablet_mode"},
		}},
	})
}

func ChromePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	params := s.Param().(chromePIPEnergyAndPowerTestParams)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(ctx)

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	var pointerController pointer.Controller
	if params.tabletMode {
		pointerController, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create touch controller: ", err)
		}
	} else {
		pointerController = pointer.NewMouseController(tconn)
	}
	defer pointerController.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for low CPU usage: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer conn.Close()

	if err := conn.Call(ctx, nil, "startVideo", params.videoFileName); err != nil {
		s.Fatal("Failed to start video: ", err)
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

	if err := pointer.Click(ctx, pointerController, webContentsView.Location.TopLeft().Add(pipButtonCenterInWebContents)); err != nil {
		s.Fatal("Failed to click/tap PIP button: ", err)
	}

	pipWindowFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if params.tabletMode {
		// Tap the PIP window in preparation for the resizing swipe. Otherwise, that
		// swipe will move the PIP window instead of resizing it.
		pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
		if err != nil {
			s.Fatal("Failed to get PIP window: ", err)
		}
		defer pipWindow.Release(ctx)
		if err := pointer.Click(ctx, pointerController, pipWindow.Location.CenterPoint()); err != nil {
			s.Fatal("Failed to tap center of PIP window: ", err)
		}
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get PIP resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)

	var resizeDragEnd coords.Point
	if params.bigPIP {
		resizeDragEnd = info.WorkArea.TopLeft()
	} else {
		resizeDragEnd = info.WorkArea.BottomRight().Sub(coords.NewPoint(1, 1))
	}
	if err := pointer.Drag(ctx, pointerController, resizeHandle.Location.CenterPoint(), resizeDragEnd, time.Second); err != nil {
		s.Fatal("Failed to drag PIP resize handle: ", err)
	}

	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location-change events to be propagated to the automation API: ", err)
	}

	pipWindowAfterResize, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP window after resize: ", err)
	}
	defer pipWindowAfterResize.Release(ctx)

	if params.bigPIP {
		maxWidth := info.WorkArea.Width / 2
		maxHeight := info.WorkArea.Height / 2
		// Expect the PIP window to have either the maximum width or the maximum
		// height, depending on how their ratio compares with 4x3.
		if maxWidth*3 <= maxHeight*4 {
			if pipWindowAfterResize.Location.Width != maxWidth {
				s.Fatalf("PIP window is %v (after resize attempt). It should have width %d", pipWindowAfterResize.Location.Size(), maxWidth)
			}
		} else {
			if pipWindowAfterResize.Location.Height != maxHeight {
				s.Fatalf("PIP window is %v (after resize attempt). It should have height %d", pipWindowAfterResize.Location.Size(), maxHeight)
			}
		}
	} else {
		// The minimum size of a Chrome PIP window is 260x146. The aspect ratio of the
		// video is 4x3, and so the minimum width 260 corresponds to a height of 195.
		if pipWindowAfterResize.Location.Width != 260 || pipWindowAfterResize.Location.Height != 195 {
			s.Fatalf("PIP window is %v. It should be (260 x 195)", pipWindowAfterResize.Location.Size())
		}
	}

	if params.tabletMode {
		// Ensure that the PIP window has no controls (like Pause) showing.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait five seconds: ", err)
		}
	} else {
		// Ensure that the PIP window has no resize shadows showing.
		if err := pointerController.Move(ctx, resizeDragEnd, info.WorkArea.TopLeft().Add(coords.NewPoint(20, 20)), time.Second); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
	}

	extraConn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer extraConn.Close()

	if err := webutil.WaitForQuiescence(ctx, extraConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait a minute: ", err)
	}
	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
