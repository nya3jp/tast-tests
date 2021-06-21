// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type arcPIPEnergyAndPowerTestParams struct {
	activityName string
	bigPIP       bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIPEnergyAndPower,
		Desc:         "Measures energy and power usage of ARC++ PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name:              "small",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: false},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "big",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: true},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "small_blend",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: false},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "big_blend",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: true},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "small_vm",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: false},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "big_vm",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "small_blend_vm",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: false},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "big_blend_vm",
			Val:               arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func PIPEnergyAndPower(ctx context.Context, s *testing.State) {
	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard event writer: ", err)
	}
	defer kw.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait for CPU to cool down: ", err)
	}

	params := s.Param().(arcPIPEnergyAndPowerTestParams)
	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.pictureinpicturevideo", params.activityName)
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)

	// The test activity enters PIP mode in onUserLeaveHint().
	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize app: ", err)
	}

	var pipWindow *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		pipWindow, err = ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
		if err != nil {
			return errors.Wrap(err, "the PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if params.bigPIP {
		// To resize the PIP window as reliably as possible,
		// use uiauto (not activity.ResizeWindow) and drag
		// from the corner (not the ARC++ PIP resize handle).
		const (
			// The resizing drag begins this far from the corner
			// outward along each dimension. This offset ensures
			// that we drag the corner and not the resize handle.
			pipCornerOffset = 5
			// After the resizing drag, we move the mouse this
			// far along each dimension, to nix resize shadows.
			nixResizeShadow = 20
		)
		workAreaTopLeft := info.WorkArea.TopLeft()
		if err := action.Combine(
			"resize the PIP window",
			mouse.Move(tconn, pipWindow.TargetBounds.TopLeft().Sub(coords.NewPoint(pipCornerOffset, pipCornerOffset)), 0),
			mouse.Press(tconn, mouse.LeftButton),
			mouse.Move(tconn, workAreaTopLeft, time.Second),
			mouse.Release(tconn, mouse.LeftButton),
			// Ensure that the PIP window will show no controls or resize shadows.
			mouse.Move(tconn, workAreaTopLeft.Add(coords.NewPoint(nixResizeShadow, nixResizeShadow)), time.Second),
		)(ctx); err != nil {
			// Ensure releasing the mouse button.
			if err := mouse.Release(tconn, mouse.LeftButton)(cleanupCtx); err != nil {
				s.Error("Failed to release the mouse button: ", err)
			}
			s.Fatal("Failed to resize the PIP window: ", err)
		}

		pipWindow, err = ash.GetWindow(ctx, tconn, pipWindow.ID)
		if err != nil {
			s.Fatal("PIP window gone after resize: ", err)
		}

		if 5*pipWindow.TargetBounds.Width <= 2*info.WorkArea.Width && 5*pipWindow.TargetBounds.Height <= 2*info.WorkArea.Height {
			s.Fatalf("Expected big PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	} else {
		if 10*pipWindow.TargetBounds.Width >= 3*info.WorkArea.Width && 10*pipWindow.TargetBounds.Height >= 3*info.WorkArea.Height {
			s.Fatalf("Expected small PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	}

	conn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// Tab away from the search box of chrome://settings, so that
	// there will be no blinking cursor.
	if err := kw.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to send Tab: ", err)
	}

	// triedToStopTracing means that cr.StopTracing(cleanupCtx)
	// was already done, with or without success (if it failed
	// then we have no reason to try again with the same timeout).
	triedToStopTracing := false
	defer func() {
		if triedToStopTracing {
			return
		}
		if _, err := cr.StopTracing(cleanupCtx); err != nil {
			s.Error("Failed to stop tracing viz.triangles in cleanup phase: ", err)
		}
	}()
	// At this time, systrace causes kernel crash on dedede devices. Because of
	// that and data points from systrace isn't actually helpful to most of
	// UI tests, disable systraces for the time being.
	// TODO(https://crbug.com/1162385, b/177636800): enable it.
	if err := cr.StartTracing(ctx, []string{"disabled-by-default-viz.triangles"}, cdputil.DisableSystrace()); err != nil {
		s.Fatal("Failed to start tracing viz.triangles: ", err)
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

	pv, err := timeline.StopRecording(ctx)
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}

	// As we still have to save results to files, we are not yet
	// focusing on cleanup, but we can safely pass cleanupCtx
	// (borrowing from the time reserved for cleanup) because
	// StopTracing was deferred to cleanup and we are now getting
	// it done ahead of time (see comment on triedToStopTracing).
	triedToStopTracing = true
	tr, err := cr.StopTracing(cleanupCtx)
	if err != nil {
		s.Fatal("Failed to stop tracing viz.triangles: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}

	if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), "trace.data.gz")); err != nil {
		s.Error("Failed to save trace data: ", err)
	}
}
