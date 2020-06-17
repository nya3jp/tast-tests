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
			Name:              "clamshell_small_low_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "clamshell_small_high_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
		}, {
			Name:              "tablet_small_low_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "tablet_mode"},
		}, {
			Name:              "tablet_small_high_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2, "tablet_mode"},
		}, {
			Name:              "clamshell_big_low_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "clamshell_big_high_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
		}, {
			Name:              "tablet_big_low_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "tablet_mode"},
		}, {
			Name:              "tablet_big_high_bit_depth",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2, "tablet_mode"},
		}},
	})
}

func hasAbsoluteValueGreaterThanOne(x int) bool {
	return x*x > 1
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

	pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP window before resize: ", err)
	}
	defer pipWindow.Release(ctx)

	if params.tabletMode {
		// Tap the PIP window in preparation for the resizing swipe. Otherwise, that
		// swipe will move the PIP window instead of resizing it.
		if err := pointer.Click(ctx, pointerController, pipWindow.Location.CenterPoint()); err != nil {
			s.Fatal("Failed to tap center of PIP window: ", err)
		}
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get PIP resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)

	var desiredPIPSize coords.Size
	if params.bigPIP {
		desiredPIPSize = coords.NewSize(400, 300)
	} else {
		desiredPIPSize = coords.NewSize(320, 240)
	}
	resizeDragStart := resizeHandle.Location.CenterPoint()
	var resizeDragEnd coords.Point
	if params.tabletMode {
		resizeDragEnd = pipWindow.Location.BottomRight().Sub(coords.NewPoint(desiredPIPSize.Width, desiredPIPSize.Height))
	} else {
		resizeDragEnd = coords.NewPoint(
			resizeDragStart.X+pipWindow.Location.Width-desiredPIPSize.Width,
			resizeDragStart.Y+pipWindow.Location.Height-desiredPIPSize.Height)
	}

	if err := pointer.Drag(ctx, pointerController, resizeDragStart, resizeDragEnd, time.Second); err != nil {
		s.Fatal("Failed to drag PIP resize handle: ", err)
	}

	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location-change events to be propagated to the automation API: ", err)
	}

	pipWindow, err = chromeui.Find(ctx, tconn, pipWindowFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP window after resize: ", err)
	}
	defer pipWindow.Release(ctx)

	if achievedPIPSize := pipWindow.Location.Size(); hasAbsoluteValueGreaterThanOne(achievedPIPSize.Width-desiredPIPSize.Width) || hasAbsoluteValueGreaterThanOne(achievedPIPSize.Height-desiredPIPSize.Height) {
		s.Fatalf("Attempted to resize PIP window to %v but achieved %v", desiredPIPSize, achievedPIPSize)
	}

	if !params.tabletMode {
		// Ensure that the PIP window will show no controls or resize shadows.
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
