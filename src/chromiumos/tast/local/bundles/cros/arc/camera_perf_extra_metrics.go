// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraPerfExtraMetrics,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures extra camera metrics such as open/close time and snapshot time",
		Contacts: []string{
			"mojahsu@chromium.org",
			"chromeos-camera-eng@google.com",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Fixture:      "arcBootedRestricted",
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val:               setup.ForceBatteryDischarge,
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val:               setup.ForceBatteryDischarge,
		}, {
			Name:              "nobatterymetrics",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val:               setup.NoBatteryDischarge,
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val:               setup.NoBatteryDischarge,
		}},
		Timeout: 10 * time.Minute,
	})
}

func CameraPerfExtraMetrics(ctx context.Context, s *testing.State) {
	const (
		cameraAppActivity         = ".CameraActivity"
		cameraAppApk              = "ArcCameraFpsTest.apk"
		cameraAppPackage          = "org.chromium.arc.testapp.camerafps"
		intentGetCameraCloseTime  = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_CLOSE_TIME"
		intentGetCameraOpenTime   = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_OPEN_TIME"
		intentGetLastSnapshotTime = "org.chromium.arc.testapp.camerafps.ACTION_GET_LAST_SNAPSHOT_TIME"
		intentResetCamera         = "org.chromium.arc.testapp.camerafps.ACTION_RESET_CAMERA"
		intentTakePhoto           = "org.chromium.arc.testapp.camerafps.ACTION_TAKE_PHOTO"
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

	sup, cleanup := setup.New("camera perf extra metrics")

	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	batteryMode := s.Param().(setup.BatteryDischargeMode)
	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
		Wifi: setup.DisableWifiInterfaces, Battery: batteryMode, NightLight: setup.DisableNightLight}))

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

	p := perf.NewValues()

	const (
		afterBootWarmupDuration = 30 * time.Second
		cameraResetCount        = 15
		// Snapshots can be really small if the room is dark, but JPEGs are never smaller than 100 bytes.
		minExpectedFileSize = 100
		snapshotCount       = 15
		snapshotWarmupCount = 5
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
			s.Fatalf("Unexpected result from intent %s: %q", intentGetCameraOpenTime, o)
		}

		closeTime := 0
		if o, err := a.BroadcastIntentGetData(ctx, intentGetCameraCloseTime); err != nil {
			s.Fatal("Could not send intent: ", err)
		} else if openTime, err = strconv.Atoi(o); err != nil {
			s.Fatalf("Unexpected result from intent %s: %q", intentGetCameraCloseTime, o)
		}

		p.Append(openCameraMetric, float64(openTime))
		p.Append(closeCameraMetric, float64(closeTime))
	}

	// Measure taking a photo (snapshot)
	s.Logf("Measure snapshot time: %d warmup rounds, %d measurements", snapshotWarmupCount, snapshotCount)
	snapshotMetric := perf.Metric{Name: "snapshot_time", Unit: "ms", Direction: perf.SmallerIsBetter, Multiple: true}

	for i := 0; i < snapshotCount+snapshotWarmupCount; i++ {
		s.Logf("Iteration %d snapshot", i)

		if outputFile, err := a.BroadcastIntentGetData(ctx, intentTakePhoto); err != nil {
			s.Error("Could not send intent: ", err)
		} else {
			filePath := filepath.Join("files/DCIM", outputFile)
			s.Log("Output file: ", filePath)
			// Check if photo file was generated.
			if fileSize, err := arc.PkgFileSize(ctx, cr.NormalizedUser(), cameraAppPackage, filePath); err != nil {
				s.Error("Could not determine size of photo file: ", err)
			} else if fileSize < minExpectedFileSize {
				s.Errorf("Photo file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
			}
		}

		if i >= snapshotWarmupCount {
			o, err := a.BroadcastIntentGetData(ctx, intentGetLastSnapshotTime)
			if err != nil {
				s.Fatal("Could not send intent: ", err)
			}

			snapshotTime, err := strconv.Atoi(o)
			if err != nil {
				s.Fatalf("Unexpected result from intent %s: %q", intentGetLastSnapshotTime, o)
			}

			if snapshotTime == -1 {
				s.Fatalf("Intent %q failed: No snapshot time was recorded", intentGetLastSnapshotTime)
			}
			p.Append(snapshotMetric, float64(snapshotTime))
		}
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
