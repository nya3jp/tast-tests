// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerCameraGcaPreviewPerf,
		Desc: "Measures the battery drain during camera preview with GCA",
		Contacts: []string{
			"springerm@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"GoogleCameraArc.apk"},
		Params: []testing.Param{{
			Name:              "",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Timeout: 45 * time.Minute,
	})
}

func PowerCameraGcaPreviewPerf(ctx context.Context, s *testing.State) {
	const (
		gcaActivity = "com.android.camera.CameraLauncher"
		gcaApk      = "GoogleCameraArc.apk"
		gcaPackage  = "com.google.android.GoogleCameraArc"

		// TODO(springerm): Make iteration count an optional command line parameter.
		iterationCount    = 30
		iterationDuration = 10 * time.Second
		warumupDuration   = 30 * time.Second
	)

	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	sup, cleanup := setup.New("GCA camera preview power")

	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, setup.ForceBatteryDischarge))

	// Install GCA APK.
	a := s.PreValue().(arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, s.DataPath(gcaApk), gcaPackage))

	// Grant permissions to activity.
	for _, permission := range []string{
		"ACCESS_COARSE_LOCATION", "ACCESS_FINE_LOCATION", "CAMERA",
		"READ_EXTERNAL_STORAGE", "RECORD_AUDIO", "WRITE_EXTERNAL_STORAGE"} {
		fullPermission := "android.permission." + permission
		sup.Add(setup.GrantAndroidPermission(ctx, a, gcaPackage, fullPermission))
	}

	// TODO(springerm): WaitUntilCPUCoolDown before starting GCA.
	// Start GCA (Google Camera App).
	sup.Add(setup.StartActivity(ctx, tconn, a, gcaPackage, gcaActivity))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))

	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	s.Log("Warmup: Waiting a bit before starting the measurement")
	if err := testing.Sleep(ctx, warumupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Starting measurement")
	if err := metrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, iterationCount*iterationDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
