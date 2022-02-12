// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apploading"
	"chromiumos/tast/local/bundles/cros/arc/nethelper"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	batteryMode       setup.BatteryDischargeMode
	binaryTranslation bool
}

var (
	// arcAppLoadingBooted is a precondition similar to arc.Booted() with no opt-in and disables some heavy Android activities that use system resources.
	arcAppLoadingBooted = arc.NewPrecondition("arcapploading_booted", nil /* GAIAVARS */, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

	// arcAppLoadingRtVcpuVMBooted adds feature to boot ARC with realtime vcpu is enabled.
	arcAppLoadingRtVcpuVMBooted = arc.NewPrecondition("arcapploading_rt_vcpu_vmbooted", nil /* GAIAVARS */, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--enable-features=ArcRtVcpuDualCore,ArcRtVcpuQuadCore")...)

	// arcAppLoadingODirectVMBooted enables O_DIRECT for crosvm.
	arcAppLoadingODirectVMBooted = arc.NewPrecondition("arcapploading_odirect_vmbooted", nil /* GAIAVARS */, nil /* GAIALOGINPOOLVARS */, true /* O_DIRECT */, append(arc.DisableSyncFlags())...)

	// arcAppLoadingDalvikMemoryProfileVMBooted enables ArcUseDalvikMemoryProfile chrome feature.
	arcAppLoadingDalvikMemoryProfileVMBooted = arc.NewPrecondition("arcapploading_dalvik_memory_profile_vmbooted", nil /* GAIAVARS */, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--enable-features=ArcUseDalvikMemoryProfile")...)
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppLoadingPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures set of apploading performance metrics and uploads them as perf metrics",
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
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "rt_vcpu_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingRtVcpuVMBooted,
		}, {
			Name:              "o_direct_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingODirectVMBooted,
		}, {
			Name:              "dalvik_memory_profile_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingDalvikMemoryProfileVMBooted,
		}, {
			Name:              "binarytranslation",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge(), hwdep.X86()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: true,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "vm_binarytranslation",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge(), hwdep.X86()),
			Val: testParameters{
				batteryMode:       setup.ForceBatteryDischarge,
				binaryTranslation: true,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.NoBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testParameters{
				batteryMode:       setup.NoBatteryDischarge,
				binaryTranslation: false,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "binarytranslation_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge(), hwdep.X86()),
			Val: testParameters{
				batteryMode:       setup.NoBatteryDischarge,
				binaryTranslation: true,
			},
			Pre: arcAppLoadingBooted,
		}, {
			Name:              "vm_binarytranslation_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge(), hwdep.X86()),
			Val: testParameters{
				batteryMode:       setup.NoBatteryDischarge,
				binaryTranslation: true,
			},
			Pre: arcAppLoadingBooted,
		}},
		VarDeps: []string{"arc.AppLoadingPerf.username", "arc.AppLoadingPerf.password"},
	})
}

// AppLoadingPerf automates app loading benchmark measurements to simulate
// system resource utilization in terms of memory, file system, networking,
// graphics, ui, etc. that can found in a game or full-featured app.  Each
// subflow will be tested separately including separate performance metrics
// uploads.  The overall final benchmark score combined and uploaded as well.
func AppLoadingPerf(ctx context.Context, s *testing.State) {
	const (
		// tbfRateMbit* specifies how fast the data will leave the primary bucket (float).
		tbfRateMbitX86 = 10
		// TODO(b/215621884): Based on ARCVM network team's manual iperf3 bandwidth and Play
		// Store game download tests on kukui vs. kukui-arc-r. Targeting simulated performance
		// where VM is at ~50% of Container. Need to verify on more ARM boards with Crosbolt data.
		tbfRateMbitArm = 1.60
		// tbfLatency is amount of time a packet can be delayed by token rate before drop (int).
		tbfLatencyMs = 18
		// tbfBurst is the size of the bucket used by rate option (int).
		tbfBurstKb = 10
	)

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

	// Add initial traffic control queuing discipline settings (b/169947243) for
	// traffic shaping based on experiments with netem, RTT latency, and iperf3
	// bandwidth measurements. Only kernel version 4.4+ supports tc-tbf.
	if ver, arch, err := sysutil.KernelVersionAndArch(); err != nil {
		s.Fatal("Failed to get kernel version: ", err)
	} else if ver.IsOrLater(4, 4) {
		var tbfRateMbit float64
		if strings.HasPrefix(arch, "x86") {
			tbfRateMbit = tbfRateMbitX86
		} else {
			tbfRateMbit = tbfRateMbitArm
		}
		if err := conn.AddTcTbf(ctx, tbfRateMbit, tbfLatencyMs, tbfBurstKb); err != nil {
			s.Fatal("Failed to add tc-tbf: ", err)
		}
	}

	finalPerfValues := perf.NewValues()
	param := s.Param().(testParameters)

	// Geometric mean for tests in the same group are computed together.  All
	// tests where group is not defined will be computed separately using the
	// geometric means from other groups.
	tests := []struct {
		name    string
		prefix  string
		subtest string
		group   string
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
		name:   "NetworkTest",
		prefix: "network",
	}, {
		name:   "OpenGLTest",
		prefix: "opengl",
	}, {
		name:   "DecompressionTest",
		prefix: "decompression",
	}, {
		name:   "UITest",
		prefix: "ui",
	}}

	// Obtain specific APK file name for the CPU architecture being tested.
	a := s.PreValue().(arc.PreData).ARC
	apkName, err := apploading.ApkNameForArch(ctx, a)
	if err != nil {
		s.Fatal("Failed to get APK name: ", err)
	}
	config := apploading.TestConfig{
		PerfValues:           finalPerfValues,
		BatteryDischargeMode: param.batteryMode,
		ApkPath:              s.DataPath(apkName),
		OutDir:               s.OutDir(),
		// Don't disable Wifi for network test since ethernet connection in lab is not guaranteed.
		// Otherwise tc-tbf settings will not be applied since it would have been disabled and reset.
		WifiInterfacesMode: setup.DoNotChangeWifiInterfaces,
	}

	// Many apps / games run with binary translation (b/169623350#comment8)
	// and thus it's an important use case to exercise.
	if param.binaryTranslation {
		config.ApkPath = s.DataPath(apploading.ArmApkName)
	}

	var scores []float64
	groups := make(map[string][]float64)
	cr := s.PreValue().(arc.PreData).Chrome
	for _, test := range tests {
		config.ClassName = test.name
		config.Prefix = test.prefix
		config.Subtest = test.subtest

		score, err := runAppLoadingTest(ctx, config, a, cr)
		if err != nil {
			s.Fatal("Failed to run apploading test: ", err)
		}

		// Put scores in the same group together, else add to top-level scores.
		if test.group != "" {
			groups[test.group] = append(groups[test.group], score)
		} else {
			scores = append(scores, score)
		}
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
	shorterCtx, cancel := context.WithTimeout(ctx, 510*time.Second)
	defer cancel()

	// Each subflow should take no longer than 8.5 minutes based on stainless
	// data. If it takes longer, very likely the app is stuck (e.g. b/169367367).
	return apploading.RunTest(shorterCtx, config, a, cr)
}
