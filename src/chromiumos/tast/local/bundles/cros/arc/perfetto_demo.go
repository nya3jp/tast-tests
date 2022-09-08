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
	"chromiumos/tast/local/arc/perfetto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoDemo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Perfetto Demo",
		Contacts:     []string{"sstan@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Data:         []string{"perfetto_config_demo.pbtxt", "ArcCompanionLibDemo.apk"},
		Timeout:      5 * time.Minute,
	})
}

func PerfettoDemo(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	if err := a.Install(ctx, s.DataPath("ArcCompanionLibDemo.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	traceResultPath := filepath.Join(s.OutDir(), "perfetto.trace")
	s.Logf("Start trace activity launching, save to %v", traceResultPath)
	if err := perfetto.Trace(ctx, a, s.DataPath("perfetto_config_demo.pbtxt"), traceResultPath, false, func(ctx context.Context) error {
		act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
		if err != nil {
			return errors.Wrap(err, "failed to launch Android Settings")
		}
		defer act.Close()

		if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
			s.Fatal("Failed to start activity: ", err)
		}
		defer act.Stop(ctx, tconn)
		time.Sleep(5 * time.Second)
		return nil
	}); err != nil {
		s.Fatal("Error on run perfetto trace")
	}

	if output, err := perfetto.Metrics(ctx, traceResultPath, perfetto.AndroidCpuMetric, perfetto.AndroidFrameTimelineMetric); err != nil {
		s.Fatal("Error on run metrics:", err)
	} else {
		s.Log("Metrics output:\n%v", string(output[:]))
	}
	s.Log("Finish trace activity launching")
}
