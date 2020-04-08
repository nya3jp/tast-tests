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
		Func: PowerCameraRecordingPerf,
		Desc: "Measures the battery drain during camera recording at 30 FPS",
		Contacts: []string{
			"springerm@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
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
		Timeout: 5 * time.Minute,
	})

	// TODO(b/153129376): Extend test to record with 30 and 60 FPS.
}

func PowerCameraRecordingPerf(ctx context.Context, s *testing.State) {
	const (
		cameraAppActivity      = ".CameraActivity"
		cameraAppApk           = "ArcCameraFpsTest.apk"
		cameraAppPackage       = "org.chromium.arc.testapp.camerafps"
		intentGetDroppedFrames = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_DROPPED_FRAMES"
		intentGetHistogram     = "org.chromium.arc.testapp.camerafps.ACTION_GET_HISTOGRAM"
		intentGetTotalFrames   = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_FRAMES"
		intentGetRecordingSize = "org.chromium.arc.testapp.camerafps.ACTION_GET_RECORDING_SIZE"
		intentResetData        = "org.chromium.arc.testapp.camerafps.ACTION_RESET_HISTOGRAM"
		intentSetFPS           = "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_FPS"
		intentStartRecording   = "org.chromium.arc.testapp.camerafps.ACTION_START_RECORDING"
		intentStopRecording    = "org.chromium.arc.testapp.camerafps.ACTION_STOP_RECORDING"
		minExpectedFileSize    = 1024 * 1024 // 1 MB
		targetFPS              = "30"
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

	sup, cleanup := setup.New("camera recording power and fps")

	defer func(ctx context.Context) {
		if err := cleanup(ctx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	sup.Add(setup.PowerTest(ctx))

	// Install camera testing app.
	a := s.PreValue().(arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, arc.APKPath(cameraAppApk), cameraAppPackage))

	// Grant permissions to activity.
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.CAMERA"))
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.RECORD_AUDIO"))
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.READ_EXTERNAL_STORAGE"))
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.WRITE_EXTERNAL_STORAGE"))

	// Start camera testing app.
	sup.Add(setup.StartActivity(ctx, tconn, a, cameraAppPackage, cameraAppActivity))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	p := perf.NewValues()

	const (
		iterationCount          = 30
		iterationDuration       = 2 * time.Second
		afterBootWarmupDuration = 30 * time.Second
		cameraWarmupDuration    = 30 * time.Second
	)

	s.Log("Warmup: Waiting for Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Set target FPS:", targetFPS)
	if _, err := a.BroadcastIntent(ctx, intentSetFPS, "--ei", "fps", targetFPS); err != nil {
		s.Fatal("Could not send intent: ", err)
	}

	resolution, err := a.BroadcastIntentGetData(ctx, intentGetRecordingSize)
	if err != nil {
		s.Fatal("Failed to query resolution from activity: ", err)
	}

	// Create metrics. We report separately for each target FPS.
	numFramesMetric := perf.Metric{Name: resolution + "_total_num_frames", Unit: "frames", Direction: perf.BiggerIsBetter}
	numDroppedFramesMetric := perf.Metric{Name: resolution + "_num_dropped_frames", Unit: "frames", Direction: perf.SmallerIsBetter}
	frameDropRatioMetric := perf.Metric{Name: resolution + "_frame_drop_ratio", Unit: "ratio", Direction: perf.SmallerIsBetter}

	powerMetrics, err := perf.NewTimelineWithPrefix(ctx, resolution+"_", power.TestMetrics()...)
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	outputFile, err := a.BroadcastIntentGetData(ctx, intentStartRecording)
	if err != nil {
		s.Fatal("Could not send intent: ", err)
	}
	s.Log("Recording to file: ", outputFile)

	s.Log("Warmup: Waiting a bit before starting the measurement")
	if err := testing.Sleep(ctx, cameraWarmupDuration); err != nil {
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
	} else if droppedFrames, err = strconv.Atoi(o); err != nil {
		s.Fatal("Unexpected result from intent " + intentGetDroppedFrames + ": " + o)
	}

	totalFrames := 0
	if o, err := a.BroadcastIntentGetData(ctx, intentGetTotalFrames); err != nil {
		s.Fatal("Could not send intent: ", err)
	} else if totalFrames, err = strconv.Atoi(o); err != nil {
		s.Fatal("Unexpected result from intent " + intentGetTotalFrames + ": " + o)
	}

	p.Set(numFramesMetric, float64(totalFrames))
	p.Set(numDroppedFramesMetric, float64(droppedFrames))

	if totalFrames == 0 {
		p.Set(frameDropRatioMetric, 0.0)
	} else {
		p.Set(frameDropRatioMetric, float64(droppedFrames)/float64(totalFrames))
	}

	// Print frame duration histogram to log file.
	hist, err := a.BroadcastIntentGetData(ctx, intentGetHistogram)
	if err != nil {
		s.Fatal("Could not send intent: ", err)
	}
	s.Logf("Frame duration histogram: %q", hist)

	if _, err = a.BroadcastIntent(ctx, intentStopRecording); err != nil {
		s.Fatal("Could not send intent: ", err)
	}

	// Check if video file was generated.
	fileSize, err := a.FileSize(ctx, outputFile)
	if err != nil {
		s.Fatal("Could not determine size of video recording: ", err)
	}

	if fileSize < minExpectedFileSize {
		s.Fatalf("Video recording file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
