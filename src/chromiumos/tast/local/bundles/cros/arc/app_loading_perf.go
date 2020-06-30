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
	"chromiumos/tast/testing"
)

const (
	arcAppLoadingGaia = &arc.GaiaVars{
		UserVar: "arc.AppLoadingPerf.username",
		PassVar: "arc.AppLoadingPerf.password",
	}
	// arcAppLoadingBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
	// that it disables some heavy post-provisioned Android activities that use system resources.
	arcAppLoadingBooted = arc.NewPrecondition("arcapploading_booted", false, arcAppLoadingGaia, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

	// arcAppLoadingVMBooted is a precondition similar to arc.VMBooted(). The only difference from arc.VMBooted() is
	// that it disables some heavy post-provisioned Android activities that use system resources.
	arcAppLoadingVMBooted = arc.NewPrecondition("arcapploading_vmbooted", true, arcAppLoadingGaia, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")
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
		Data:         []string{"ArcAppLoadingTest.apk"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arcAppLoadingBooted,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arcAppLoadingVMBooted,
		}},
	})
}

// AppLoadingPerf automates app loading benchmark measurements to simulate
// system resource utilization in terms of memory, file system, networking,
// graphics, ui, etc. that can found in a game or full-featured app.  Each
// subflow will be tested separately including separate performance metrics
// uploads.  The overall final benchmark score combined and uploaded as well.
func AppLoadingPerf(ctx context.Context, s *testing.State) {
	// TODO: Reevaluate weights for different tests (alanding)
	var weightsDict = map[string]float64{
		"memory":  1.0,
		"file":    1.0,
		"network": 1.0,
		"opengl":  1.0,
		"ui":      1.0,
	}

	finalPerfValues := perf.NewValues()
	// TODO: Add more configs as soon as the APK class names / tests are also added.
	configs := []apploading.testConfig{{
		className:  "MemoryTest",
		prefix:     "memory",
		perfValues: finalPerfValues,
	}, {
		className:  "FileTest",
		prefix:     "file",
		perfValues: finalPerfValues,
	}, {
		className:  "NetworkTest",
		prefix:     "network",
		perfValues: finalPerfValues,
	}}

	var float finalScore
	for _, config := range configs {
		score, err := apploading.RunTest(ctx, s, config)
		if err != nil {
			s.Fatal("Failed to run apploading test: ", err)
		}

		weight, ok := weightsDict[config.prefix]
		if !ok {
			s.Fatal("Failed to obtain weight value for test: ", weightDict.prefix)
		}
		score *= weight
		finalScore += score
	}

	finalPerfValues.Set(
		perf.Metric{
			Name:      "final_score",
			Unit:      "points",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, finalScore)

	if err := finalPerfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save final perf metrics: ", err)
	}
}
