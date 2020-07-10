// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIPEnergyAndPower,
		Desc:         "Measures energy and power usage of ARC++ PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "small",
			Val:  false,
		}, {
			Name: "big",
			Val:  true,
		}},
	})
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func isPIP(w *ash.Window) bool { return w.State == ash.WindowStatePIP }

// computeInsets returns a formal Rect on which Left, Top, Right(), and Bottom() are the insets from before to after.
func computeInsets(before, after coords.Rect) coords.Rect {
	return coords.NewRectLTRB(
		after.Left-before.Left,
		after.Top-before.Top,
		before.Right()-after.Right(),
		before.Bottom()-after.Bottom())
}

// getInsetBounds is based on the Java function by the same name at
// https://source.corp.google.com/pi-arc-dev/frameworks/base/services/core/java/com/android/server/wm/PinnedStackController.java
func getInsetBounds(mWorkspaceBounds, mWorkspaceInsets coords.Rect, mScreenEdgeInsets coords.Point) coords.Rect {
	return coords.NewRectLTRB(
		mWorkspaceBounds.Left+mWorkspaceInsets.Left+mScreenEdgeInsets.X,
		mWorkspaceBounds.Top+mWorkspaceInsets.Top+mScreenEdgeInsets.Y,
		mWorkspaceBounds.Right()-mWorkspaceInsets.Right()-mScreenEdgeInsets.X,
		mWorkspaceBounds.Bottom()-mWorkspaceInsets.Bottom()-mScreenEdgeInsets.Y)
}

// getSizeForAspectRatio is based on the Java function by the same name at
// https://source.corp.google.com/pi-arc-dev/frameworks/base/core/java/com/android/internal/policy/PipSnapAlgorithm.java
func getSizeForAspectRatio(aspectRatio, minEdgeSize float64, displayWidth, displayHeight int) coords.Size {
	const mDefaultSizePercent = 0.23
	const mMaxAspectRatioForMinSize = 1.777778
	const mMinAspectRatioForMinSize = 1.0 / mMaxAspectRatioForMinSize

	smallestDisplaySize := minInt(displayWidth, displayHeight)
	minSize := int(math.Max(minEdgeSize, float64(smallestDisplaySize)*mDefaultSizePercent))

	var width int
	var height int
	if aspectRatio <= mMinAspectRatioForMinSize || aspectRatio > mMaxAspectRatioForMinSize {
		// Beyond these points, we can just use the min size as the shorter edge
		if aspectRatio <= 1.0 {
			// Portrait, width is the minimum size
			width = minSize
			height = int(math.Round(float64(width) / aspectRatio))
		} else {
			// Landscape, height is the minimum size
			height = minSize
			width = int(math.Round(float64(height) * aspectRatio))
		}
	} else {
		// Within these points, we ensure that the bounds fit within the radius of the limits
		// at the points
		widthAtMaxAspectRatioForMinSize := mMaxAspectRatioForMinSize * float64(minSize)
		radius := math.Hypot(widthAtMaxAspectRatioForMinSize, float64(minSize))
		height = int(math.Round(math.Sqrt((radius * radius) / (aspectRatio*aspectRatio + 1.0))))
		width = int(math.Round(float64(height) * aspectRatio))
	}
	return coords.NewSize(width, height)
}

// getMinimumSizeForPip is based on the Java function by the same name at
// https://source.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
func getMinimumSizeForPip(mPipMinEdgeSize int, aspectRatio float64, pipInsetBounds coords.Rect) coords.Size {
	return getSizeForAspectRatio(aspectRatio, float64(mPipMinEdgeSize), pipInsetBounds.Width, pipInsetBounds.Height)
}

// getMaximumSizeForPip is based on the Java function by the same name at
// https://source.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
func getMaximumSizeForPip(pipInsetBounds coords.Rect) coords.Size {
	return coords.NewSize(pipInsetBounds.Width/2, pipInsetBounds.Height/2)
}

func PIPEnergyAndPower(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode of the primary display: ", err)
	}

	pointerController := pointer.NewMouseController(tconn)
	defer pointerController.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for low CPU usage: ", err)
	}

	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.pictureinpicturevideo", ".VideoActivity")
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := act.SetWindowState(ctx, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize app: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.FindWindow(ctx, tconn, isPIP); err != nil {
			return errors.Wrap(err, "The PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	pipWindow, err := ash.FindWindow(ctx, tconn, isPIP)
	if err != nil {
		s.Fatal("PIP window not found, even though it was discovered in polling just now: ", err)
	}

	pxPerDp := displayMode.DeviceScaleFactor
	dpPerPx := 1.0 / pxPerDp
	// Argument to Java functions getMinimumSizeForPip and getMaximumSizeForPip at
	// https://source.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
	pipInsetBounds := getInsetBounds(
		info.Bounds.WithUnitConversion(pxPerDp),
		computeInsets(info.Bounds, info.WorkArea).WithUnitConversion(pxPerDp),
		coords.NewPoint(16, 16).WithUnitConversion(pxPerDp))

	if s.Param().(bool) { // big PIP window
		workAreaTopLeft := info.WorkArea.TopLeft()
		if err := pointerController.Move(ctx, workAreaTopLeft, pipWindow.TargetBounds.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse to PIP window: ", err)
		}

		resizeHandleBounds, err := d.Object(
			ui.ClassName("android.widget.ImageView"),
			ui.DescriptionMatches("(?!.+)"),
			ui.PackageName("com.android.systemui"),
		).GetBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get bounds of PIP resize handle: ", err)
		}

		if err := pointer.Drag(ctx, pointerController, resizeHandleBounds.WithUnitConversion(dpPerPx).CenterPoint(), workAreaTopLeft, time.Second); err != nil {
			s.Fatal("Failed to drag PIP resize handle: ", err)
		}

		if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for location-change events to be propagated to the automation API: ", err)
		}

		pipWindow, err = ash.GetWindow(ctx, tconn, pipWindow.ID)
		if err != nil {
			s.Fatal("PIP window gone after resize: ", err)
		}

		maxSize := getMaximumSizeForPip(pipInsetBounds).WithUnitConversion(dpPerPx)
		// Expect the PIP window to have either the maximum width or the maximum
		// height, depending on how their ratio compares with 4x3.
		if maxSize.Width*3 <= maxSize.Height*4 {
			if pipWindow.TargetBounds.Width != maxSize.Width {
				s.Fatalf("PIP window is %v (after resize attempt). It should have width %d", pipWindow.TargetBounds.Size(), maxSize.Width)
			}
		} else {
			if pipWindow.TargetBounds.Height != maxSize.Height {
				s.Fatalf("PIP window is %v (after resize attempt). It should have height %d", pipWindow.TargetBounds.Size(), maxSize.Height)
			}
		}
	} else { // small PIP window
		minSize := getMinimumSizeForPip(int(math.Round(108.0*pxPerDp)), 4.0/3.0, pipInsetBounds).WithUnitConversion(dpPerPx)
		actualSize := pipWindow.TargetBounds.Size()
		if !actualSize.Equals(minSize) {
			s.Fatalf("PIP window is %v. It should be %v", actualSize, minSize)
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
