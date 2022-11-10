// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoundedDisplayPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures performance of rounded display",
		Contacts:     []string{"yichenz@google.com", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{Name: "rounded_display_on", Val: true},
			{Name: "rounded_display_off", Val: false},
		},
	})
}

func RoundedDisplayPerf(ctx context.Context, s *testing.State) {
	const mouseMoveDuration = 3 * time.Second

	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var opts []chrome.Option
	if s.Param().(bool) {
		opts = append(opts, chrome.EnableFeatures("kRoundedDisplay"))
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to create chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Ensure clamshell mode is enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	// Ensure landscape orientation.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the internal display info: ", err)
		}
		if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Open 4 chrome windows to cover the whole screen.
	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 4); err != nil {
		s.Fatal("Failed to open 4 browser windows: ", err)
	}
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	if len(ws) != 4 {
		s.Fatalf("Unexpected number of windows: got %d; want 4", len(ws))
	}
	displayInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	workArea := displayInfo.WorkArea
	displayBounds := displayInfo.Bounds
	s.Log("Display bounds: ", displayBounds)
	windowBounds := []coords.Rect{
		coords.NewRectLTRB(workArea.Left, workArea.Top, workArea.CenterX(), workArea.CenterY()),
		coords.NewRectLTRB(workArea.CenterX(), workArea.Top, workArea.Right(), workArea.CenterY()),
		coords.NewRectLTRB(workArea.Left, workArea.CenterY(), workArea.CenterX(), workArea.Bottom()),
		coords.NewRectLTRB(workArea.CenterX(), workArea.CenterY(), workArea.Right(), workArea.Bottom()),
	}
	for i, bound := range windowBounds {
		if err := ash.SetWindowStateAndWait(ctx, tconn, ws[i].ID, ash.WindowStateNormal); err != nil {
			s.Fatal("Failed to set window state normal: ", err)
		}
		if _, _, err := ash.SetWindowBounds(ctx, tconn, ws[i].ID, bound, displayInfo.ID); err != nil {
			s.Fatal("Failed to set window bounds: ", err)
		}
	}
	s.Fatal("faile")

	// Open a Files window.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	defer files.Close(cleanupCtx)
	titleBar, err := files.Info(ctx, nodewith.ClassName("WebAppFrameToolbarView"))
	if err != nil {
		s.Log(uiauto.RootDebugInfo(ctx, tconn))
		s.Fatal("Failed to obtain Files app info: ", err)
	}
	startDragPt := titleBar.Location.CenterPoint()

	// Verify that there is only one window, and get its ID.
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the windows: ", err)
	}
	if len(ws) != 5 {
		s.Fatalf("Unexpected number of windows: got %d; want 5", len(ws))
	}
	wID := ws[0].ID

	recorder, err := cujrecorder.NewRecorder(ctx, cr, tconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)
	if err := recorder.AddCommonMetrics(tconn, tconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// 1. Drag a window around the periphery of the display.
		if err := pc.Drag(startDragPt,
			pc.DragTo(displayBounds.TopRight(), mouseMoveDuration),
			pc.DragTo(displayBounds.BottomRight(), mouseMoveDuration),
			pc.DragTo(displayBounds.BottomLeft(), mouseMoveDuration),
			pc.DragTo(displayBounds.TopLeft(), mouseMoveDuration),
			pc.DragTo(displayBounds.TopRight(), mouseMoveDuration))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag the window")
		}

		// 2. Enter and exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enter overview mode")
		}
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview mode")
		}

		// 3. Maximize the window.
		if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMaximized); err != nil {
			return errors.Wrap(err, "failed to set window state to Maximized")
		}

		// 4. Move the cursor around edge of the display.
		if err := uiauto.Combine(
			"move the cursor around the periphery of the internal display",
			mouse.Move(tconn, displayBounds.BottomRight(), mouseMoveDuration),
			mouse.Move(tconn, displayBounds.BottomLeft(), mouseMoveDuration),
			mouse.Move(tconn, displayBounds.TopLeft(), mouseMoveDuration),
			mouse.Move(tconn, displayBounds.TopRight(), mouseMoveDuration),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the cursor")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
