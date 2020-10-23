// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apploading"
	"chromiumos/tast/local/bundles/cros/arc/nethelper"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

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
		Data:         []string{apploading.X86ApkName, apploading.ArmApkName},
		Timeout:      25 * time.Minute,
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
	// Start network helper to serve requests from the app.
	conn, err := nethelper.Start(ctx, apploading.NethelperPort)
	if err != nil {
		s.Fatal("Failed to start nethelper: ", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			s.Logf("WARNING: Failed to close nethelper connection: %s", err)
		}
	}()

	finalPerfValues := perf.NewValues()
	batteryMode := s.Param().(setup.BatteryDischargeMode)

	// Geometric mean for tests in the same group are computed together.  All
	// tests where group is not defined will be computed separately using the
	// geometric means from other groups.
	tests := []struct {
		name      string
		prefix    string
		subtest   string
		group     string
		multiarch bool
	}{{
		name:   "MemoryTest",
		prefix: "memory",
	}, {
		name:    "FileTest",
		prefix:  "file_obb",
		subtest: "runObbTest",
		group:   "not_ext4_fs",
	}, {
		name:    "FileTest",
		prefix:  "file_squashfs",
		subtest: "runSquashFSTest",
		group:   "not_ext4_fs",
	}, {
		name:    "FileTest",
		prefix:  "file_esd",
		subtest: "runEsdTest",
		group:   "not_ext4_fs",
	}, {
		name:    "FileTest",
		prefix:  "file_ext4",
		subtest: "runExt4Test",
	}, {
		name:      "NetworkTest",
		prefix:    "network",
		multiarch: true,
	}, {
		name:   "OpenGLTest",
		prefix: "opengl",
	}, {
		name:   "DecompressionTest",
		prefix: "decompression",
	}, {
		name:      "UITest",
		prefix:    "ui",
		multiarch: true,
	}}

	a := s.PreValue().(arc.PreData).ARC
	apkName, err := apploading.ApkNameForArch(ctx, a)
	if err != nil {
		s.Fatal("Failed to get APK name: ", err)
	}
	config := apploading.TestConfig{
		PerfValues:           finalPerfValues,
		BatteryDischargeMode: batteryMode,
		ApkPath:              s.DataPath(apkName),
		OutDir:               s.OutDir(),
	}

	var firstErr error
	var scores []float64
	groups := make(map[string][]float64)
	cr := s.PreValue().(arc.PreData).Chrome
	for _, test := range tests {
		config.ClassName = test.name
		config.Prefix = test.prefix
		config.Subtest = test.subtest

		// TODO(b/169367367): Many apps / games run libhoudini (b/169446394) which
		// is a major use case. These subflows test for crashes but are not scored.
		if apkName == apploading.X86ApkName && test.multiarch {
			armConfig := config
			armConfig.ApkPath = s.DataPath(apploading.ArmApkName)
			armConfig.Prefix += "_arm"
			if _, err := runAppLoadingTest(ctx, armConfig, a, cr); err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					s.Fatalf("Failed to run %s (arm): %s", test.name, err)
				} else if firstErr == nil {
					firstErr = errors.Wrapf(err, "failed to complete %s (arm), timeout occurred", test.name)
				}
			}
		}
		score, err := runAppLoadingTest(ctx, config, a, cr)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				s.Fatalf("Failed to run %s: %s", test.name, err)
			} else if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to complete %s, timeout occurred", test.name)
			}
		}

		// Put scores in the same group together, else add to top-level scores.
		if test.group != "" {
			groups[test.group] = append(groups[test.group], score)
		} else {
			scores = append(scores, score)
		}
	}

	if firstErr != nil {
		// Previously COF but exit now that all tests are complete.
		s.Fatal("Failed to run apploading test, first error: ", firstErr)
	}

	// Obtain geometric mean of each group and append to top-level scores.
	for _, group := range groups {
		score, err := calcGeometricMean(group)
		if err != nil {
			s.Fatal("Failed to process geometric mean: ", err)
		}
		scores = append(scores, score)
	}

	// Calculate grand mean (geometric) of top-level scores which includes the
	// geometric means from each group.
	totalScore, err := calcGeometricMean(scores)
	if err != nil {
		s.Fatal("Failed to process geometric mean: ", err)
	}

	finalPerfValues.Set(
		perf.Metric{
			Name:      "total_score",
			Unit:      "None",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, totalScore)
	s.Logf("Finished all tests with total score: %.2f", totalScore)

	s.Log("Uploading perf metrics")

	if err := finalPerfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save final perf metrics: ", err)
	}
}

// calcGeometricMean computes the geometric mean but use antilog method to
// prevent overflow: EXP((LOG(x1) + LOG(x2) + LOG(x3)) ... + LOG(xn)) / n)
func calcGeometricMean(scores []float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New("scores can not be empty")
	}

	var mean float64
	for _, score := range scores {
		mean += math.Log(score)
	}
	mean /= float64(len(scores))

	return math.Exp(mean), nil
}

// runAppLoadingTest will test each app loading subflow with timeout.
func runAppLoadingTest(ctx context.Context, config apploading.TestConfig, a *arc.ARC, cr *chrome.Chrome) (float64, error) {
	shorterCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Each subflow should take no longer than 5 minutes based on stainless data.
	// If it takes longer, very likely the app is stuck (e.g. b/169367367).
	// TODO(b/169341324): Reduce subflow test timeout and overall context timeout.
	return apploading.RunTest(shorterCtx, config, a, cr)
}
