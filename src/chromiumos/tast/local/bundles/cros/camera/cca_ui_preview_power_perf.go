// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type ccaUIPreviewPowerPerfParams struct {
	batteryMode setup.BatteryDischargeMode
	appType     testutil.CCAAppType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewPowerPerf,
		Desc:         "Opens CCA and measures battery drain during preview",
		Contacts:     []string{"springerm@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         []string{"cca_ui.js"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Params: []testing.Param{{
			Name:              "noarc",
			Pre:               testutil.ChromeWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               testutil.ARCWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               testutil.ARCWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			Name:              "noarc_nobatterymetrics",
			Pre:               testutil.ChromeWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			Name:              "nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               testutil.ARCWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               testutil.ARCWithPlatformApp(),
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.PlatformApp,
			},
		}, {
			Name:              "noarc_swa",
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Pre:               testutil.ChromeWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.SWA,
			},
		}, {
			Name:              "swa",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Pre:               testutil.ARCWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.SWA,
			},
		}, {
			Name:              "vm_swa",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Pre:               testutil.ARCWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.ForceBatteryDischarge,
				appType:     testutil.SWA,
			},
		}, {
			Name:              "noarc_nobatterymetrics_swa",
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Pre:               testutil.ChromeWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.SWA,
			},
		}, {
			Name:              "nobatterymetrics_swa",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Pre:               testutil.ARCWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.SWA,
			},
		}, {
			Name:              "vm_nobatterymetrics_swa",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Pre:               testutil.ARCWithSWA(),
			Val: ccaUIPreviewPowerPerfParams{
				batteryMode: setup.NoBatteryDischarge,
				appType:     testutil.SWA,
			},
		}},
		Timeout: 5 * time.Minute,
	})
}

// CCAUIPreviewPowerPerf measures battery drain during CCA preview.
// To allow for a fair comparison with arc.PowerCameraPreviewPerf, ARCVM is running
// in the background in the vm subtest. (But CCA is a built-in ChromeOS application.)
func CCAUIPreviewPowerPerf(ctx context.Context, s *testing.State) {
	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		cr = s.PreValue().(arc.PreData).Chrome
	}
	useSWA := s.Param().(ccaUIPreviewPowerPerfParams).appType == testutil.SWA

	tb, err := testutil.NewTestBridge(ctx, cr, false)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	sup, cleanup := setup.New("CCA camera preview power")

	defer func(ctx context.Context) {
		if err := cleanup(ctx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	batteryMode := s.Param().(ccaUIPreviewPowerPerfParams).batteryMode
	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
		Wifi: setup.DisableWifiInterfaces, Battery: batteryMode}))

	const (
		iterationCount          = 30
		iterationDuration       = 2 * time.Second
		warumupDuration         = 30 * time.Second
		afterBootWarmupDuration = 30 * time.Second
	)

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	s.Log("Warmup: Waiting for ChromeOS/Android to settle down")
	if err := testing.Sleep(ctx, afterBootWarmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))

	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	// Start Chrome Camera App (CCA).
	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb, useSWA)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	if err := app.MaximizeWindow(ctx); err != nil {
		s.Fatal("Failed to maximize CCA: ", err)
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

	values, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := values.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
