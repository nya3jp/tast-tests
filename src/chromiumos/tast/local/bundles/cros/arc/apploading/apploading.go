// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apploading provides functions to assist with instrumenting and uploading
// performance metrics for ARC apploading tasts.
package apploading

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// TestConfig defines input params for apploading.RunTest function.
type TestConfig struct {
	ClassName            string
	Prefix               string
	Subtest              string
	PerfValues           *perf.Values
	BatteryDischargeMode setup.BatteryDischargeMode
	ApkPath              string
	OutDir               string
	WifiInterfacesMode   setup.WifiInterfacesMode
}

const (
	// NethelperPort is the port used for nethelper to listen for connections.
	NethelperPort = 1235

	// X86ApkName is the name of the ArcAppLoadingTest APK for x86/x86_64 devices.
	X86ApkName = "ArcAppLoadingTest_x86.apk"
	// ArmApkName is the name of the ArcAppLoadingTest APK for Arm devices.
	ArmApkName = "ArcAppLoadingTest_arm.apk"
)

// Used to keep information for a key, identified by the array of possible suffixes.
var keyInfo = []struct {
	// Possible suffixes for the key, for example "_score"
	suffixes []string
	// Unit name, for example "us"
	unitName string
	// Performance direction, for example perf.BiggerIsBetter.
	direction perf.Direction
}{{
	suffixes:  []string{"_score"},
	unitName:  "mbps",
	direction: perf.BiggerIsBetter,
},
}

// ApkNameForArch gets the name of the APK file to install on the DUT.
func ApkNameForArch(ctx context.Context, a *arc.ARC) (string, error) {
	out, err := a.Command(ctx, "getprop", "ro.product.cpu.abi").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get abi: %v", err)
	}

	if strings.HasPrefix(string(out), "x86") {
		return X86ApkName, nil
	}
	return ArmApkName, nil
}

// coolDownConfig returns the config to wait for the machine to cooldown for AppLoading tests.
// This overrides the default config timeout (5 minutes) and temperature threshold (46 C)
// settings to reduce test flakes on low-end devices.
func coolDownConfig() cpu.CoolDownConfig {
	cdConfig := cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)
	cdConfig.PollTimeout = 7 * time.Minute
	cdConfig.TemperatureThreshold = 61000
	return cdConfig
}

// RunTest executes subset of tests in APK determined by the test class name.
func RunTest(ctx context.Context, config TestConfig, a *arc.ARC, cr *chrome.Chrome) (retScore float64, retErr error) {
	const (
		packageName            = "org.chromium.arc.testapp.apploading"
		tPowerSnapshotDuration = 5 * time.Second
	)

	testName := packageName + "." + config.ClassName
	if config.Subtest != "" {
		testName += "#" + config.Subtest
	}

	testing.ContextLog(ctx, "Starting setup")

	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Some configuration actions need a test connection to Chrome.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to test API")
	}

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("apploading")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil && retErr == nil {
			retErr = errors.Wrap(err, "failed to cleanup after creating test")
		}
	}()

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
		Wifi: config.WifiInterfacesMode, Battery: config.BatteryDischargeMode, NightLight: setup.DisableNightLight}))
	if err := sup.Check(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to setup power test")
	}

	testing.ContextLogf(ctx, "Installing APK: %s", config.ApkPath)
	sup.Add(setup.InstallApp(ctx, a, config.ApkPath, packageName))
	if err := sup.Check(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to install apk app")
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Prefix(config.Prefix+"_"), perf.Interval(tPowerSnapshotDuration))
	if err != nil {
		return 0, errors.Wrap(err, "failed to build metrics")
	}
	testing.ContextLog(ctx, "Finished setup")

	// Drop caches before starting test,
	if err := disk.DropCaches(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to drop caches")
	}

	testing.ContextLog(ctx, "Waiting until CPU is stabilized")
	if err := cpu.WaitUntilStabilized(ctx, coolDownConfig()); err != nil {
		return 0, errors.Wrap(err, "failed to wait until CPU is stabilized")
	}

	testing.ContextLogf(ctx, "Running test: %s", testName)
	if err := metrics.Start(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to start metrics")
	}

	if err := metrics.StartRecording(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to start recording")
	}

	out, err := a.Command(ctx, "am", "instrument", "-w", "-e", "class", testName, packageName).CombinedOutput()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute test")
	}

	outputFile := filepath.Join(config.OutDir, config.Prefix+"_test_log.txt")
	if err := ioutil.WriteFile(outputFile, []byte(out), 0644); err != nil {
		return 0, errors.Wrapf(err, "failed to save test output: %s", outputFile)
	}
	testing.ContextLog(ctx, "Finished writing to log: ", outputFile)

	// Make sure test is completed successfully.
	if !regexp.MustCompile(`\nOK \(\d+ tests?\)\n*$`).Match(out) {
		return 0, errors.Errorf("test is not completed successfully, see: %s", outputFile)
	}

	powerPerfValues, err := metrics.StopRecording(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error while recording power metrics")
	}

	// Merge previous perf metrics with new power metrics.
	config.PerfValues.Merge(powerPerfValues)

	testing.ContextLog(ctx, "Analyzing results")

	// total up all score from the test
	var score float64

	// Output may be prepended by other chars, and order of elements is not defined.
	// Examples:
	// INSTRUMENTATION_STATUS: MemoryTest_score=7834091.30
	// .INSTRUMENTATION_STATUS: MemoryTest_byte_count=230989
	// org.chromium.arc.testapp.apploading.ArcAppLoadTest:INSTRUMENTATION_STATUS: FileTest_duration=239890435.78
	for _, m := range regexp.MustCompile(`INSTRUMENTATION_STATUS: (.+?)=(\d+.?\d*)`).FindAllStringSubmatch(string(out), -1) {
		key := m[1]
		value, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return score, errors.Wrap(err, "failed to parse float")
		}
		if strings.HasSuffix(key, "_score") {
			score += value
			info, err := makeMetricInfo(key)
			if err != nil {
				return score, errors.Wrap(err, "failed to parse key")
			}
			config.PerfValues.Set(info, value)
		}
	}

	var result int
	// There may be several INSTRUMENTATION_STATUS_CODE: X (x = 0 or x = -1)
	for _, m := range regexp.MustCompile(`INSTRUMENTATION_STATUS_CODE: (-?\d+)`).FindAllStringSubmatch(string(out), -1) {
		if val, err := strconv.Atoi(m[1]); err != nil {
			return score, errors.Wrapf(err, "failed to convert %q to integer", m[1])
		} else if val == -1 {
			result = val
			break
		}
	}
	testing.ContextLogf(ctx, "Finished test with result: %d", result)

	if result != -1 {
		return score, errors.New("failed to pass instrumentation test")
	}

	return score, nil
}

// makeMetricInfo creates a metric description that can be supplied for reporting with the actual
// value. Returns an error in case key is not recognized.
func makeMetricInfo(key string) (perf.Metric, error) {
	for _, ki := range keyInfo {
		for _, suffix := range ki.suffixes {
			if !strings.HasSuffix(key, suffix) {
				continue
			}
			return perf.Metric{
				Name:      key,
				Unit:      ki.unitName,
				Direction: ki.direction,
				Multiple:  false,
			}, nil
		}
	}

	return perf.Metric{}, errors.Errorf("key could not be recognized: %s", key)
}
