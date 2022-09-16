// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowResizePerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures animation smoothness of resizing a window",
		Contacts:     []string{"xiyuan@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Val:     browser.TypeAsh,
				Fixture: "chromeLoggedIn",
			}, {
				Name:              "lacros",
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           "lacros",
			},
		},
	})
}

func WindowResizePerf(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

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
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)
	}

	metricsName := "Ash.InteractiveWindowResize.TimeToPresent"
	if s.Param().(browser.Type) == browser.TypeLacros {
		metricsName = "Ash.InteractiveWindowResize.Lacros.TimeToPresent"
	}

	runner := perfutil.NewRunner(cr.Browser())
	for i, numWindows := range []int{1, 2} {
		conn, err := cs.NewConn(ctx, ui.PerftestURL, browser.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open a new connection: ", err)
		}
		defer conn.Close()

		// This must be done after opening a new window to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(browser.Type) == browser.TypeLacros {
			if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
				s.Fatal("Failed to close blank tab: ", err)
			}
		}

		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil || len(ws) == 0 {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		if len(ws) != numWindows {
			s.Errorf("The number of windows mismatches: %d vs %d", len(ws), numWindows)
			continue
		}
		id0 := ws[0].ID
		if err := ash.SetWindowStateAndWait(ctx, tconn, id0, ash.WindowStateLeftSnapped); err != nil {
			s.Fatalf("Failed to set the state of window (%d): %v", id0, err)
		}
		if len(ws) > 1 {
			id1 := ws[1].ID
			if err := ash.SetWindowStateAndWait(ctx, tconn, id1, ash.WindowStateRightSnapped); err != nil {
				s.Fatalf("Failed to set the state of window (%d): %v", id1, err)
			}
		}

		suffix := fmt.Sprintf("%dwindows", numWindows)
		runner.RunMultiple(ctx, suffix, uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			w0, err := ash.GetWindow(ctx, tconn, id0)
			if err != nil {
				s.Error("Failed to get windows: ", err)
			}
			bounds := w0.BoundsInRoot

			start := coords.NewPoint(bounds.Right(), bounds.CenterY())
			if len(ws) > 1 {
				// For multiple windows; hover on the boundary, wait for the resize-handle
				// to appear, and move onto the resize handle.
				if err := mouse.Move(tconn, start, 0)(ctx); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
				// Waiting for the resize-handle to appear. TODO(mukai): find the right
				// wait to see its visibility.
				if err := testing.Sleep(ctx, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				// 20 DIP would be good enough to move the drag handle.
				start.Y += 20
				if err := mouse.Move(tconn, start, 0)(ctx); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
			} else {
				if err := mouse.Move(tconn, start, 0)(ctx); err != nil {
					return errors.Wrap(err, "failed to move the mouse")
				}
			}
			left := coords.NewPoint(start.X-bounds.Width/4, start.Y)
			right := coords.NewPoint(start.X+bounds.Width/4, start.Y)

			if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to press the button")
			}
			released := false
			defer func() {
				if !released {
					mouse.Release(tconn, mouse.LeftButton)
				}
			}()
			if err := mouse.Move(tconn, left, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "faeild to drag to the left")
			}
			if err := mouse.Move(tconn, right, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag to the right")
			}
			if err := mouse.Move(tconn, start, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag back to the start position")
			}
			if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to release the left button")
			}
			released = true
			return testing.Sleep(ctx, time.Second)
		}, metricsName)),
			perfutil.StoreAll(perf.SmallerIsBetter, "ms", suffix))
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
