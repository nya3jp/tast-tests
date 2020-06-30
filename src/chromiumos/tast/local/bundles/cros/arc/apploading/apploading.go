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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Used to define input params for RunTest function.
type testConfig struct {
	className  string
	prefix     string
	perfValues *perf.Values
}

// Used to keep information for a key, identified by the array of possible suffixes.
var keyInfo = []struct {
	// Possible suffixes for the key, for example "_score"
	suffixes []string
	// Unit name, for example "us"
	unitName string
	// Performance direction, for example perf.BiggerIsBetter.
	direction perf.Direction
}{{
	suffixes:  []string{"_duration", "_utime", "_stime"},
	unitName:  "us",
	direction: perf.SmallerIsBetter,
}, {
	suffixes:  []string{"_byte_count"},
	unitName:  "bytes",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_score"},
	unitName:  "points",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_page_faults", "_page_reclaims", "_fs_reads", "_fs_writes", "_context_switches"},
	unitName:  "counts",
	direction: perf.SmallerIsBetter,
}, {
	suffixes:  []string{"_msgs_sent", "_msgs_rcvd", ""},
	unitName:  "messages",
	direction: perf.SmallerIsBetter,
},
}

// RunTest executes subset of tests in ArcAppLoadTest.apk determined by the test class name.
func RunTest(ctx context.Context, s *testing.State, config testConfig) (float64, error) {
	const (
		packageName            = "org.chromium.arc.testapp.apploading"
		apkName                = "ArcAppLoadingTest.apk"
		tPowerSnapshotDuration = 5 * time.Second
	)

	var float64 score
	perfValues := config.perfValues
	a := s.PreValue().(arc.PreData).ARC
	cr = s.PreValue().(arc.PreData).Chrome

	// Clear RAM memory cache, buffer, and swap space before every test.
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		return score, errors.Wrap(err, "failed to flush buffers")
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		return score, errors.New("failed to clear caches")
	}
	if err := testexec.CommandContext(ctx, "swapoff -a && swapon -a").Run(testexec.DumpLogOnError); err != nil {
		return score, errors.New("ailed to clear swap space")
	}

	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	shortCtx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Some configuration actions need a test connection to Chrome.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return score, errors.Wrap(err, "failed to connect to test API")
	}

	s.Log("Waiting until CPU is idle")

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return score, errors.Wrap(err, "failed to wait until CPU is idle")
	}

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("apploading")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			return score, errors.New("cleanup failed")
		}
	}()

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, tconn))
	if err := sup.Check(ctx); err != nil {
		return score, errors.Wrap(err, "failed to setup power test")
	}

	s.Log("Installing: ", apkName)

	sup.Add(setup.InstallApp(ctx, a, s.DataPath(apkName), packageName))
	if err := sup.Check(ctx); err != nil {
		return score, errors.Wrap(err, "failed to install apk app")
	}

	// Grant permissions to activity.
	sup.Add(setup.GrantAndroidPermission(ctx, a, packageName, "android.permission.READ_EXTERNAL_STORAGE"))
	sup.Add(setup.GrantAndroidPermission(ctx, a, packageName, "android.permission.WRITE_EXTERNAL_STORAGE"))

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Prefix(config.prefix+"_"), perf.Interval(tPowerSnapshotDuration))
	if err != nil {
		return score, errors.Wrap(err, "failed to build metrics")
	}
	s.Log("Finished setup")

	s.Log("Waiting until CPU is cool down")
	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		return score, errors.Wrap(err, "failed to cool down")
	}

	s.Log("Running test")
	out, err := a.Command(ctx, "am", "instrument", "-w", "-e", "class", packageName+"."+config.classNAme, packageName).CombinedOutput()
	if err != nil {
		return score, errors.Wrap(err, "failed to execute test")
	}

	outputFile := filepath.Join(s.OutDir(), config.prefix+"_test_log.txt")
	err = ioutil.WriteFile(outputFile, []byte(out), 0644)
	if err != nil {
		return score, errors.Wrapf(err, "failed to save test output: %s", outputFile)
	}

	if err := metrics.Start(ctx); err != nil {
		return score, errors.Wrap(err, "failed to start metrics")
	}

	if err := metrics.StartRecording(ctx); err != nil {
		return score, errors.Wrap(err, "failed to start recording")
	}

	// Make sure test is completed successfully.
	if !regexp.MustCompile(`\nOK \(\d+ tests?\)\n*$`).Match(out) {
		return score, errors.New("test is not completed successfully, see: " + outputFile)
	}

	powerPerfValues, err := metrics.StopRecording()
	if err != nil {
		return score, errors.Wrap(err, "error while recording power metrics")
	}

	s.Log("Analyzing results")

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
		}
		info, err := makeMetricInfo(key)
		if err != nil {
			return score, errors.Wrap(err, "failed to parse key")
		}
		perfValues.Set(info, value)
	}

	perfValues.Merge(powerPerfValues)

	var int result
	// There may be several INSTRUMENTATION_STATUS_CODE: X (x = 0 or x = -1)
	for _, m := range regexp.MustCompile(`INSTRUMENTATION_STATUS_CODE: (-?\d+)`).FindAllStringSubmatch(string(out), -1) {
		if val, err := strconv.Atoi(m[1]); err != nil {
			return score, errors.Wrapf(err, "failed to convert %q to integer", m[1])
		}
		if result == -1 {
			result = val
			break
		}
	}

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

	return perf.Metric{}, errors.New("key could not be recognized: " + key)
}
