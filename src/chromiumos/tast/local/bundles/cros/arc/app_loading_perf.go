// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apploading"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const apkName = "ArcAppLoadingTest.apk"

var (
	arcAppLoadingGaia = &arc.GaiaVars{
		UserVar: "arc.AppLoadingPerf.username",
		PassVar: "arc.AppLoadingPerf.password",
	}

	// arcAppLoadingBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
	// that it disables some heavy post-provisioned Android activities that use system resources.
	arcAppLoadingBooted = arc.NewPrecondition("arcapploading_booted", arcAppLoadingGaia, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

	// arcAppLoadingVMBooted is a precondition similar to arc.VMBooted(). The only difference from arc.VMBooted() is
	// that it disables some heavy post-provisioned Android activities that use system resources.
	arcAppLoadingVMBooted = arc.NewPrecondition("arcapploading_vmbooted", arcAppLoadingGaia, "--ignore-arcvm-dev-conf", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AppLoadingPerf,
		Desc: "Captures set of apploading performance metrics and uploads them as perf metrics",
		Contacts: []string{
			"alanding@chromium.org",
			"khmel@chromium.org",
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{apkName},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val:               setup.ForceBatteryDischarge,
			Pre:               arcAppLoadingBooted,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val:               setup.ForceBatteryDischarge,
			Pre:               arcAppLoadingVMBooted,
		}, {
			Name:              "nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val:               setup.NoBatteryDischarge,
			Pre:               arcAppLoadingBooted,
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val:               setup.NoBatteryDischarge,
			Pre:               arcAppLoadingVMBooted,
		}},
		Vars: []string{"arc.AppLoadingPerf.username", "arc.AppLoadingPerf.password"},
	})
}

// AppLoadingPerf automates app loading benchmark measurements to simulate
// system resource utilization in terms of memory, file system, networking,
// graphics, ui, etc. that can found in a game or full-featured app.  Each
// subflow will be tested separately including separate performance metrics
// uploads.  The overall final benchmark score combined and uploaded as well.
func AppLoadingPerf(ctx context.Context, s *testing.State) {
	weightsDict := map[string]float64{
		"memory":  0.1,
		"file":    6.5,
		"network": 5.2,
		"opengl":  4.0,
	}

	finalPerfValues := perf.NewValues()
	batteryMode := s.Param().(setup.BatteryDischargeMode)
	tests := []struct {
		name   string
		prefix string
	}{{
		name:   "MemoryTest",
		prefix: "memory",
	}, {
		name:   "FileTest",
		prefix: "file",
	}, {
		name:   "NetworkTest",
		prefix: "network",
	}, {
		name:   "OpenGLTest",
		prefix: "opengl",
	}}

	config := apploading.TestConfig{
		PerfValues:           finalPerfValues,
		BatteryDischargeMode: batteryMode,
		ApkPath:              s.DataPath(apkName),
		OutDir:               s.OutDir(),
	}

	var totalScore float64
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	for _, test := range tests {
		config.ClassName = test.name
		config.Prefix = test.prefix
		score, err := apploading.RunTest(ctx, config, a, cr)
		if err != nil {
			s.Fatal("Failed to run apploading test: ", err)
		}

		weight, ok := weightsDict[config.Prefix]
		if !ok {
			s.Fatal("Failed to obtain weight value for test: ", config.Prefix)
		}
		score *= weight
		totalScore += score
	}

	finalPerfValues.Set(
		perf.Metric{
			Name:      "total_score",
			Unit:      "mbps",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, totalScore)
	s.Logf("Finished all tests with total score: %.2f", totalScore)

	s.Log("Uploading perf metrics")

	if err := finalPerfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save final perf metrics: ", err)
	}
}
