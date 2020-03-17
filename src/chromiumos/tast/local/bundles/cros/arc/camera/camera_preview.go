// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package camera contains helper functions for CameraPerfExtraMetrics and PowerCameraPreviewPerf*Fps.
package camera

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// PowerCameraPreviewPerf measures power usage and achieved FPS.
func PowerCameraPreviewPerf(ctx context.Context, s *testing.State, targetFps string) {
	const (
		cameraAppActivity      = ".CameraActivity"
		cameraAppApk           = "ArcCameraFpsTest.apk"
		cameraAppPackage       = "org.chromium.arc.testapp.camerafps"
		intentGetDroppedFrames = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_DROPPED_FRAMES"
		intentGetTotalFrames   = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_FRAMES"
		intentResetData        = "org.chromium.arc.testapp.camerafps.ACTION_RESET_HISTOGRAM"
		intentSetFps           = "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_FPS"
	)

	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	sup, cleanup := setup.New("camera preview power and fps")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx))

	// Install camera testing app.
	a := s.PreValue().(arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, arc.APKPath(cameraAppApk), cameraAppPackage))

	// Grant permissions to activity.
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.CAMERA"))

	// TODO(springerm): WaitUntilCPUCoolDown before starting activity.
	// Start camera testing app.
	sup.Add(setup.StartActivity(ctx, a, cameraAppPackage, cameraAppActivity))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	p := perf.NewValues()

	s.Log("Finished setup")

	const (
		// TODO(springerm): Make iteration count an optional command line parameter.
		iterationCount            = 30
		iterationDuration         = 2 * time.Second
		afterBootWarumupDuration  = 30 * time.Second
		cameraResetWarmupDuration = 15 * time.Second
	)

	s.Log("Warmup: Waiting for Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarumupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Set target FPS: " + targetFps + " FPS")

	// Create metrics. We report separately for each target FPS.
	numFramesMetric := perf.Metric{Name: "total_num_frames", Unit: "frames", Direction: perf.BiggerIsBetter}
	numDroppedFramesMetric := perf.Metric{Name: "num_dropped_frames", Unit: "frames", Direction: perf.SmallerIsBetter}
	frameDropRatioMetric := perf.Metric{Name: "frame_drop_ratio", Unit: "ratio", Direction: perf.SmallerIsBetter}

	powerMetrics, err := perf.NewTimeline(ctx, power.TestMetrics()...)
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if _, err = a.BroadcastIntent(ctx, intentSetFps, "--ei", "fps", targetFps); err != nil {
		s.Fatal("Could not send intent: ", err)
	}

	s.Log("Warmup: Waiting a bit before starting the measurement")
	if err := testing.Sleep(ctx, cameraResetWarmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Starting measurement")

	if _, err = a.BroadcastIntent(ctx, intentResetData); err != nil {
		s.Fatal("Could not send intent: ", err)
	}

	// Keep camera running and record power usage.
	for i := 0; i < iterationCount; i++ {
		if err := testing.Sleep(ctx, iterationDuration); err != nil {
			s.Fatal("Failed to sleep between metric snapshots: ", err)
		}
		s.Logf("Iteration %d snapshot", i)
		if err := powerMetrics.Snapshot(ctx, p); err != nil {
			s.Fatal("Failed to snapshot metrics: ", err)
		}
	}

	droppedFrames := 0
	if o, err := a.BroadcastIntentGetData(ctx, intentGetDroppedFrames); err != nil {
		s.Fatal("Could not send intent: ", err)
	} else {
		if droppedFrames, err = strconv.Atoi(o); err != nil {
			s.Fatal("Unexpected result from intent " + intentGetDroppedFrames + ": " + o)
		}
	}

	totalFrames := 0
	if o, err := a.BroadcastIntentGetData(ctx, intentGetTotalFrames); err != nil {
		s.Fatal("Could not send intent: ", err)
	} else {
		if totalFrames, err = strconv.Atoi(o); err != nil {
			s.Fatal("Unexpected result from intent " + intentGetTotalFrames + ": " + o)
		}
	}

	p.Append(numFramesMetric, float64(totalFrames))
	p.Append(numDroppedFramesMetric, float64(droppedFrames))

	if totalFrames == 0 {
		p.Append(frameDropRatioMetric, 0.0)
	} else {
		p.Append(frameDropRatioMetric, float64(droppedFrames)/float64(totalFrames))
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
