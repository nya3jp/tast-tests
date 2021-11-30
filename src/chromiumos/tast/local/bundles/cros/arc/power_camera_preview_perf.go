// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type powerCameraPreviewPerfArgs struct {
	targetFPS     string
	dischargeMode setup.BatteryDischargeMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerCameraPreviewPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the battery drain and camera statistics (e.g., dropped frames) during camera preview at 30/60 FPS",
		Contacts: []string{
			"mojahsu@chromium.org",
			"chromeos-camera-eng@google.com",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Fixture:      "arcBootedWithDisableSyncFlags",
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Params: []testing.Param{
			{
				Name:              "30fps",
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
				Val: powerCameraPreviewPerfArgs{
					targetFPS:     "30",
					dischargeMode: setup.ForceBatteryDischarge,
				},
			},
			{
				Name:              "30fps_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
				Val: powerCameraPreviewPerfArgs{
					targetFPS:     "30",
					dischargeMode: setup.ForceBatteryDischarge,
				},
			},
			{
				Name:              "30fps_nobatterymetrics",
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
				Val: powerCameraPreviewPerfArgs{
					targetFPS:     "30",
					dischargeMode: setup.NoBatteryDischarge,
				},
			},
			{
				Name:              "30fps_vm_nobatterymetrics",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
				Val: powerCameraPreviewPerfArgs{
					targetFPS:     "30",
					dischargeMode: setup.NoBatteryDischarge,
				},
			},
		},
		Timeout: 10 * time.Minute,
	})
}

func PowerCameraPreviewPerf(ctx context.Context, s *testing.State) {
	const (
		cameraAppActivity      = ".CameraActivity"
		cameraAppApk           = "ArcCameraFpsTest.apk"
		cameraAppPackage       = "org.chromium.arc.testapp.camerafps"
		intentGetDroppedFrames = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_DROPPED_FRAMES"
		intentGetHistogram     = "org.chromium.arc.testapp.camerafps.ACTION_GET_HISTOGRAM"
		intentGetPreviewSize   = "org.chromium.arc.testapp.camerafps.ACTION_GET_PREVIEW_SIZE"
		intentGetTotalFrames   = "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_FRAMES"
		intentResetData        = "org.chromium.arc.testapp.camerafps.ACTION_RESET_HISTOGRAM"
		intentSetFps           = "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_FPS"
	)

	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	sup, cleanup := setup.New("camera preview power and fps")

	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	args := s.Param().(powerCameraPreviewPerfArgs)
	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
		Wifi: setup.DisableWifiInterfaces, Battery: args.dischargeMode, NightLight: setup.DisableNightLight}))

	// Install camera testing app.
	a := s.FixtValue().(*arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, arc.APKPath(cameraAppApk), cameraAppPackage))

	// Grant permissions to activity.
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.CAMERA"))

	// Wait until CPU is cooled down.
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		s.Error("CPU failed to cool down: ", err)
	}

	// Start camera testing app.
	sup.Add(setup.StartActivity(ctx, tconn, a, cameraAppPackage, cameraAppActivity))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	const (
		// TODO(springerm): Make iteration count an optional command line parameter.
		iterationCount            = 30
		iterationDuration         = 2 * time.Second
		afterBootWarmupDuration   = 30 * time.Second
		cameraResetWarmupDuration = 30 * time.Second
	)

	s.Log("Warmup: Waiting for Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Logf("Set target FPS: %v FPS", args.targetFPS)
	if _, err = a.BroadcastIntent(ctx, intentSetFps, "--ei", "fps", args.targetFPS); err != nil {
		s.Fatal("Could not send intent: ", err)
	}

	resolution, err := a.BroadcastIntentGetData(ctx, intentGetPreviewSize)
	if err != nil {
		s.Fatal("Failed to query resolution from activity: ", err)
	}
	s.Log("Camera preview resolution: ", resolution)

	// Create metrics. We report separately for each target FPS.
	numFramesMetric := perf.Metric{Name: "total_num_frames", Unit: "frames", Direction: perf.BiggerIsBetter}
	numDroppedFramesMetric := perf.Metric{Name: "num_dropped_frames", Unit: "frames", Direction: perf.SmallerIsBetter}
	frameDropRatioMetric := perf.Metric{Name: "frame_drop_ratio", Unit: "ratio", Direction: perf.SmallerIsBetter}

	powerMetrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
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
	if err := powerMetrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, iterationCount*iterationDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := powerMetrics.StopRecording(ctx)
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
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
		s.Fatal("Camera app did not receive any frames")
	} else {
		p.Set(frameDropRatioMetric, float64(droppedFrames)/float64(totalFrames))
	}

	// Print frame duration histogram to log file.
	o, err := a.BroadcastIntentGetData(ctx, intentGetHistogram)
	if err != nil {
		s.Fatal("Could not send intent: ", err)
	}
	s.Logf("Frame duration histogram: %q", o)

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
