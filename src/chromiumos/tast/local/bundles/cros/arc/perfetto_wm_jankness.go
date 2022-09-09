// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoWMJankness,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Detects jank while an ARC window changes its size",
		Contacts:     []string{"yukashu@google.com", "sstan@google.com", "brpol@google.com"},
		// This test currently only work for ARC T, due to no ARC T board running tast
		// test, not add this test to any group.
		// TODO(sstan): Add it to mainline once ARC T launched.
		SoftwareDeps: []string{"android_vm", "chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"config.pbtxt"},
		Timeout:      5 * time.Minute,
	})
}

// PerfettoWMJankness will gather tracing while the app's window changes its size, and
// analyze the trace result file.
func PerfettoWMJankness(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Start trace activity launching")

	// Overwrite tracing_on flag in kernel tracefs.
	if err := a.ForceEnableTrace(ctx); err != nil {
		s.Fatal(err, "Error on enabling perfetto trace")
	}

	// Run the perfetto basing on config,and pull the trace result. This will return after
	// perfetto finish tracing or get error during tracing.
	if err := a.PerfettoTrace(ctx, s.DataPath("config.pbtxt"), filepath.Join(s.OutDir(), "pulledtrace"), false, func(ctx context.Context) error {

		act, err := arc.NewActivity(a, "com.android.settings", ".Settings")

		if err != nil {
			return errors.Wrap(err, "failed to launch Android Settings")
		}
		defer act.Close()

		if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to start activity")
		}
		defer act.Stop(ctx, tconn)

		if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
			return err
		}
		// Maximize the window of the app.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventMaximize); err != nil {
			return err
		}
		// Wait 3 seconds for the animation to run.
		// 3 seconds should be long enough to run the animation on any device.
		testing.Sleep(ctx, 3*time.Second)

		// Normalize the window of the app.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
			return err
		}
		// Wait 3 seconds for the animation to run.
		testing.Sleep(ctx, 3*time.Second)

		return nil
	}); err != nil {
		s.Fatal("Error on run perfetto trace")
	}
	s.Log("Finish trace activity launching")
	// TODO(sstan): Analyze trace result file
}
