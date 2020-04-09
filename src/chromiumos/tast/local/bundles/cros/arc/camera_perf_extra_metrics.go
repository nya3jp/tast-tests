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
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CameraPerfExtraMetrics,
		Desc: "Measures extra camera metrics such as open/close time and snapshot time",
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
}

func CameraPerfExtraMetrics(ctx context.Context, s *testing.State) {
	const (
		cameraAppActivity        = ".CameraActivity"
		cameraAppApk             = "ArcCameraFpsTest.apk"
		cameraAppPackage         = "org.chromium.arc.testapp.camerafps"
		intentGetCameraCloseTime = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_CLOSE_TIME"
		intentGetCameraOpenTime  = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_OPEN_TIME"
		intentResetCamera        = "org.chromium.arc.testapp.camerafps.ACTION_RESET_CAMERA"
		intentTakePhoto          = "org.chromium.arc.testapp.camerafps.ACTION_TAKE_PHOTO"
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

	sup, cleanup := setup.New("camera perf extra metrics")

	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn))

	// Install camera testing app.
	a := s.PreValue().(arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, arc.APKPath(cameraAppApk), cameraAppPackage))

	// Grant permissions to activity.
	sup.Add(setup.GrantAndroidPermission(ctx, a, cameraAppPackage, "android.permission.CAMERA"))

	// TODO(springerm): WaitUntilCPUCoolDown before starting activity.
	// Start camera testing app.
	sup.Add(setup.StartActivity(ctx, tconn, a, cameraAppPackage, cameraAppActivity))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	p := perf.NewValues()

	const (
		afterBootWarmupDuration = 30 * time.Second
		cameraResetCount        = 15
		snapshotCount           = 15
		snapshotWarmupCount     = 5
	)

	s.Log("Warmup: Waiting for Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarmupDuration); err != nil {
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
		if o, err := a.BroadcastIntentGetData(ctx, intentGetCameraOpenTime); err != nil {
			s.Fatal("Could not send intent: ", err)
		} else if openTime, err = strconv.Atoi(o); err != nil {
			s.Fatal("Unexpected result from intent " + intentGetCameraOpenTime + ": " + o)
		}

		closeTime := 0
		if o, err := a.BroadcastIntentGetData(ctx, intentGetCameraCloseTime); err != nil {
			s.Fatal("Could not send intent: ", err)
		} else if openTime, err = strconv.Atoi(o); err != nil {
			s.Fatal("Unexpected result from intent " + intentGetCameraCloseTime + ": " + o)
		}

		p.Append(openCameraMetric, float64(openTime))
		p.Append(closeCameraMetric, float64(closeTime))
	}

	// Measure taking a photo (snapshot)
	s.Logf("Measure snapshot time: %d warmup rounds, %d measurements", snapshotWarmupCount, snapshotCount)
	snapshotMetric := perf.Metric{Name: "snapshot_time", Unit: "ms", Direction: perf.SmallerIsBetter, Multiple: true}

	for i := 0; i < snapshotCount+snapshotWarmupCount; i++ {
		s.Logf("Iteration %d snapshot", i)

		snapshotTime := 0
		if o, err := a.BroadcastIntentGetData(ctx, intentTakePhoto); err != nil {
			s.Fatal("Could not send intent: ", err)
		} else if snapshotTime, err = strconv.Atoi(o); err != nil {
			s.Fatal("Unexpected result from intent " + intentTakePhoto + ": " + o)
		}

		if i > snapshotWarmupCount {
			p.Append(snapshotMetric, float64(snapshotTime))
		}
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
