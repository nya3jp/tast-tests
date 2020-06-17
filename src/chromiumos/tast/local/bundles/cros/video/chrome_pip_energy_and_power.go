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
	videoFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPEnergyAndPower,
		Desc:         "Measures energy and power usage of Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"pip.html"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:      "clamshell_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "clamshell_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "clamshell_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "clamshell_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "clamshell_vp9_hdr",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData: []string{"peru.8k.cut.hdr.vp9.webm"},
			Pre:       pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "clamshell_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_vp9_sw_hdr",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData: []string{"peru.8k.cut.hdr.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "clamshell_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "clamshell_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:      "tablet_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "tablet_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "tablet_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "tablet_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "tablet_vp9_hdr",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData: []string{"peru.8k.cut.hdr.vp9.webm"},
			Pre:       pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "tablet_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_vp9_sw_hdr",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData: []string{"peru.8k.cut.hdr.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "tablet_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "tablet_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
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

	energyMetrics := power.NewRAPLMetrics()
	if err := energyMetrics.Setup(ctx, "chrome_pip_energy_"); err != nil {
		s.Fatal("Failed to set up energy metrics: ", err)
	}

	powerMetrics := power.NewRAPLPowerMetrics()
	if err := powerMetrics.Setup(ctx, "chrome_pip_power_"); err != nil {
		s.Fatal("Failed to set up power metrics: ", err)
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

	if err := pointerController.Press(ctx, webContentsView.Location.TopLeft().Add(pipButtonCenterInWebContents)); err != nil {
		s.Fatal("Failed to press PIP button: ", err)
	}
	if err := pointerController.Release(ctx); err != nil {
		s.Fatal("Failed to release PIP button: ", err)
	}

	pipWindowFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, time.Minute); err != nil {
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
		if err := pointerController.Press(ctx, pipWindow.Location.CenterPoint()); err != nil {
			s.Fatal("Failed to press center of PIP window: ", err)
		}
		if err := pointerController.Release(ctx); err != nil {
			s.Fatal("Failed to release center of PIP window: ", err)
		}
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get PIP resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)
	resizeHandleCenter := resizeHandle.Location.CenterPoint()
	if err := pointerController.Press(ctx, resizeHandleCenter); err != nil {
		s.Fatal("Failed to press PIP resize handle: ", err)
	}
	if err := pointerController.Move(ctx, resizeHandleCenter, coords.Point{X: 0, Y: 0}, time.Second); err != nil {
		s.Fatal("Failed to drag PIP resize handle: ", err)
	}
	if err := pointerController.Release(ctx); err != nil {
		s.Fatal("Failed to release PIP resize handle: ", err)
	}

	extraConn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer extraConn.Close()

	if err := webutil.WaitForQuiescence(ctx, extraConn, time.Minute); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
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
