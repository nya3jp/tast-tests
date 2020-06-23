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
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

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
			Name: "noarc_platform_app",
			Val:  cca.ChromeConfig{},
		}, {
			Name:              "platform_app",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: cca.ChromeConfig{
				ARCEnabled: true,
			},
		}, {
			Name:              "vm_platform_app",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: cca.ChromeConfig{
				ARCEnabled: true,
			},
		}, {
			Name: "noarc_swa",
			Val: cca.ChromeConfig{
				InstallSWA: true,
			},
		}, {
			Name:              "swa",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: cca.ChromeConfig{
				InstallSWA: true,
				ARCEnabled: true,
			},
		}, {
			Name:              "vm_swa",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: cca.ChromeConfig{
				InstallSWA: true,
				ARCEnabled: true,
			},
		}},
		Timeout: 5 * time.Minute,
	})
}

// CCAUIPreviewPowerPerf measures battery drain during CCA preview.
// To allow for a fair comparison with arc.PowerCameraPreviewPerf, ARCVM is running
// in the background in the vm subtest. (But CCA is a native ChromeOS application.)
func CCAUIPreviewPowerPerf(ctx context.Context, s *testing.State) {
	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	chromeConfig := s.Param().(cca.ChromeConfig)
	env, err := cca.SetupTestEnvironment(ctx, chromeConfig)
	if err != nil {
		s.Fatal("Failed to open chrome: ", err)
	}
	defer env.TearDown(ctx)

	cr := env.Chrome
	defer cr.Close(ctx)

	var a *arc.ARC
	if chromeConfig.ARCEnabled {
		a, err = arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
	}
	closeARC := func(a *arc.ARC) {
		if a != nil {
			a.Close()
		}
	}
	defer closeARC(a)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	sup, cleanup := setup.New("CCA camera preview power")

	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, setup.ForceBatteryDischarge))

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
	app, err := cca.New(ctx, env, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}

	defer app.Close(ctx)

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

	p, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
