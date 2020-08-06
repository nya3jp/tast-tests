// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowResizePerf,
		Desc:         "Measures animation smoothness of resizing a window",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func WindowResizePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Ensures in the landscape orientation; the following test scenario won't
	// succeed when the device is in the portrait mode.
	if orientation, err := display.GetOrientation(ctx, tconn); err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	} else if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain internal display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
	}

	runner := perfutil.NewRunner(cr)
	for _, numWindows := range []int{1, 2} {
		conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open a new connection: ", err)
		}
		defer conn.Close()

		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil || len(ws) == 0 {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		if len(ws) != numWindows {
			s.Errorf("The number of windows mismatches: %d vs %d", len(ws), numWindows)
			continue
		}
		id0 := ws[0].ID
		if _, err = ash.SetWindowState(ctx, tconn, id0, ash.WMEventSnapLeft); err != nil {
			s.Fatalf("Failed to set the state of window (%d): %v", id0, err)
		}
		if len(ws) > 1 {
			if _, err = ash.SetWindowState(ctx, tconn, ws[1].ID, ash.WMEventSnapRight); err != nil {
				s.Fatalf("Failed to set the state of window (%d): %v", ws[1].ID, err)
			}
		}

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		suffix := fmt.Sprintf("%dwindows", numWindows)
		runner.RunMultiple(ctx, s, suffix, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			w0, err := ash.GetWindow(ctx, tconn, id0)
			if err != nil {
				s.Error("Failed to get windows: ", err)
			}
			bounds := w0.BoundsInRoot

			start := coords.NewPoint(bounds.Right(), bounds.CenterY())
			if len(ws) > 1 {
				// For multiple windows; hover on the boundary, wait for the resize-handle
				// to appear, and move onto the resize handle.
				if err := mouse.Move(ctx, tconn, start, 0); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
				// Waiting for the resize-handle to appear. TODO(mukai): find the right
				// wait to see its visibility.
				if err := testing.Sleep(ctx, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				// 20 DIP would be good enough to move the drag handle.
				start.Y += 20
				if err := mouse.Move(ctx, tconn, start, 0); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
			} else {
				if err := mouse.Move(ctx, tconn, start, 0); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
			}
			left := coords.NewPoint(start.X-bounds.Width/4, start.Y)
			right := coords.NewPoint(start.X+bounds.Width/4, start.Y)

			if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
				return errors.Wrap(err, "failed to press the button")
			}
			released := false
			defer func() {
				if !released {
					mouse.Release(ctx, tconn, mouse.LeftButton)
				}
			}()
			if err := mouse.Move(ctx, tconn, left, time.Second/2); err != nil {
				return errors.Wrap(err, "faeild to drag to the left")
			}
			if err := mouse.Move(ctx, tconn, right, time.Second); err != nil {
				return errors.Wrap(err, "failed to drag to the right")
			}
			if err := mouse.Move(ctx, tconn, start, time.Second/2); err != nil {
				return errors.Wrap(err, "failed to drag back to the start position")
			}
			if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
				return errors.Wrap(err, "failed to release the left button")
			}
			released = true
			return testing.Sleep(ctx, time.Second)
		}, "Ash.InteractiveWindowResize.TimeToPresent"),
			perfutil.StoreAll(perf.SmallerIsBetter, "ms", suffix))
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
