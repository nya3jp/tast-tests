// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

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

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerCameraPreviewPerf,
		Desc: "Measures the battery drain and camera statistics (e.g., dropped frames) during camera preview",
		Contacts: []string{
			"springerm@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android"},
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

func PowerCameraPreviewPerf(ctx context.Context, s *testing.State) {
	const (
		cameraAppActivity        = ".CameraActivity"
		cameraAppApk             = "ArcCameraFpsTest.apk"
		cameraAppPackage         = "org.chromium.arc.testapp.camerafps"
		intentGetCameraCloseTime = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_CLOSE_TIME"
		intentGetCameraOpenTime  = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_OPEN_TIME"
		intentGetHistogram       = "org.chromium.arc.testapp.camerafps.ACTION_GET_HISTOGRAM"
		intentGetDroppedFrames   = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_DROPPED_FRAMES"
		intentGetTotalFrames     = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_FRAMES"
		intentResetCamera        = "org.chromium.arc.testapp.camerafps.ACTION_RESET_CAMERA"
		intentResetData          = "org.chromium.arc.testapp.camerafps.ACTION_RESET_HISTOGRAM"
		intentSetFps             = "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_FPS"
		intentTakePhoto          = "org.chromium.arc.testapp.camerafps.ACTION_TAKE_PHOTO"
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
		cameraResetCount          = 15
		snapshotCount             = 15
		snapshotWarmupCount       = 5
	)

	s.Log("Warmup: Waiting for Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarumupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Measure camera open and close time.
	s.Log("Measure camera open/close time")
	openCameraMetric := perf.Metric{Name: "open_camera_time", Unit: "ms", Direction: perf.SmallerIsBetter, Multiple: true}
	closeCameraMetric := perf.Metric{Name: "close_camera_time", Unit: "ms", Direction: perf.SmallerIsBetter, Multiple: true}
	for i := 0; i < cameraResetCount; i++ {
		s.Logf("Iteration %d snapshot", i)

		if _, err := a.BroadcastIntent(ctx, intentResetCamera); err != nil {
			s.Fatal("Could not send intent: ", err)
		}

		openTime := 0
		if o, err := a.BroadcastIntent(ctx, intentGetCameraOpenTime); err == nil {
			if openTime, err = strconv.Atoi(o); err != nil {
				s.Fatal("Unexpected result from intent " + intentGetCameraOpenTime + ": " + o)
			}
		} else {
			s.Fatal("Could not send intent: ", err)
		}

		closeTime := 0
		if o, err := a.BroadcastIntent(ctx, intentGetCameraCloseTime); err == nil {
			if openTime, err = strconv.Atoi(o); err != nil {
				s.Fatal("Unexpected result from intent " + intentGetCameraCloseTime + ": " + o)
			}
		} else {
			s.Fatal("Could not send intent: ", err)
		}

		p.Append(openCameraMetric, float64(openTime))
		p.Append(closeCameraMetric, float64(closeTime))
	}

	// Measure taking a photo (snapshot)
	s.Logf("Measure snapshot time: %d warmup rounds, %d measurements", snapsnotWarmupCount, snapshotCount)
	snapshotMetric := perf.Metric{Name: "snapshot_time", Unit: "ms", Direction: perf.SmallerIsBetter, Multiple: true}

	for i := 0; i < snapshotCount+snapshotWarmupCount; i++ {
		s.Logf("Iteration %d snapshot", i)

		snapshotTime := 0
		if o, err := a.BroadcastIntent(ctx, intentTakePhoto); err == nil {
			if snapshotTime, err = strconv.Atoi(o); err != nil {
				s.Fatal("Unexpected result from intent " + intentTakePhoto + ": " + o)
			}
		} else {
			s.Fatal("Could not send intent: ", err)
		}

		if i > snapshotWarmupCount {
			p.Append(snapshotMetric, float64(snapshotTime))
		}
	}

	// Measure camera statistics for various FPS targets.
	// Note: Some cameras do not support 60 FPS and won't capture more than 30 FPS, regardless of the target FPS.
	for _, targetFps := range []string{"30", "60"} {
		s.Log("Set target FPS: " + targetFps + " FPS")

		// Create metrics. We report separately for each target FPS.
		numFramesMetric := perf.Metric{Name: targetFps + "fps_total_num_frames", Unit: "frames", Direction: perf.BiggerIsBetter, Multiple: true}
		numDroppedFramesMetric := perf.Metric{Name: targetFps + "fps_num_dropped_frames", Unit: "frames", Direction: perf.SmallerIsBetter, Multiple: true}
		frameDropRatioMetric := perf.Metric{Name: targetFps + "fps_frame_drop_ratio", Unit: "ratio", Direction: perf.SmallerIsBetter, Multiple: true}

		powerMetrics, err := perf.NewTimelineWithPrefix(ctx, targetFps+"fps_", power.TestMetrics()...)
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
		if o, err := a.BroadcastIntent(ctx, intentGetDroppedFrames); err == nil {
			if droppedFrames, err = strconv.Atoi(o); err != nil {
				s.Fatal("Unexpected result from intent " + intentGetDroppedFrames + ": " + o)
			}
		} else {
			s.Fatal("Could not send intent: ", err)
		}

		totalFrames := 0
		if o, err := a.BroadcastIntent(ctx, intentGetTotalFrames); err == nil {
			if totalFrames, err = strconv.Atoi(o); err != nil {
				s.Fatal("Unexpected result from intent " + intentGetTotalFrames + ": " + o)
			}
		} else {
			s.Fatal("Could not send intent: ", err)
		}

		p.Append(numFramesMetric, float64(totalFrames))
		p.Append(numDroppedFramesMetric, float64(droppedFrames))
		p.Append(frameDropRatioMetric, float64(droppedFrames)/float64(totalFrames))
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
