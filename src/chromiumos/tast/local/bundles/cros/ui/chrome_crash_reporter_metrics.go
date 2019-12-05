// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// crashReporterHistogramName is the name of the histogram that records (among
	// other things) how many Chrome crashes were picked up by the crash_reporter.
	crashReporterHistogramName = "Platform.CrOSEvent"

	// missedCrashHistogramBucket is the bucket inside the CrashReporterHistogramName
	// histogram that counts cases where crash_reporter was not invoked for a
	// Chrome crash.
	missedCrashHistogramBucket = 26

	// callFromKernelBucket is the bucket inside the CrashReporterHistogramName
	// histogram that counts the number of Chrome crashes noticed by the kernel.
	callFromKernelBucket = 25
)

// params contains the test parameters which are different between the "miss" test
// and the "success" test.
type params struct {
	// chromeOptions gives the list of options we pass to chrome.New. These are used
	// to force a failure in the "miss" test/
	chromeOptions []chrome.Option
	// crashFileType tells chromecrash.KillAndGetCrashFiles what type of files to
	// wait on. As a side effect of the way we force a miss in the "miss" test,
	// we expect breakpad dmp files instead of the normal .meta files.
	crashFileType chromecrash.CrashFileType
	// expectMissing tells the test if we expect the missedCrashHistogramBucket
	// to get an event. This is the main point of the test -- "miss" expects an
	// event in the missedCrashHistogramBucket and "success" does not.
	expectMissing bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashReporterMetrics,
		Desc:     "Checks that anomaly detector reports whether crash_reporter was invoked",
		Contacts: []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "miss",
			Val: params{
				// By not adding the "chrome.CrashNormalMode()" option, we rely on the
				// crash handling that bypasses the crash_reporter, thus anomaly_detector
				// should count the crash as a missed crash. (See code in
				// chrome.restartChromeForTesting, where it sets CHROME_HEADLESS and
				// BREAKPAD_DUMP_LOCATION environmental variables)
				chromeOptions: []chrome.Option{chrome.ExtraArgs(chromecrash.VModuleFlag), chrome.KeepState()},
				crashFileType: chromecrash.BreakpadDmp,
				expectMissing: true,
			},
		}, {
			Name: "success",
			Val: params{
				chromeOptions: []chrome.Option{chrome.CrashNormalMode(), chrome.ExtraArgs(chromecrash.VModuleFlag), chrome.KeepState()},
				crashFileType: chromecrash.MetaFile,
				expectMissing: false,
			},
		}},
	})
}

func ChromeCrashReporterMetrics(ctx context.Context, s *testing.State) {
	// Make sure anomaly_detector isn't running, and then clear pending metrics.
	// This ensures there's no events from before the test starts.
	if err := upstart.StopJob(ctx, "anomaly-detector"); err != nil {
		s.Error("Upstart couldn't stop anomaly-detector: ", err)
	}
	// See ClearHistogramTransferFile for more about why this is needed.
	if err := metrics.ClearHistogramTransferFile(); err != nil {
		s.Error("Could not truncate existing metrics files: ", err)
	}

	params := s.Param().(params)

	cr, err := chrome.New(ctx, params.chromeOptions...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	if err = crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Could not restart anomaly detector: ", err)
	}
	oldHistogram, err := metrics.GetHistogram(ctx, cr, crashReporterHistogramName)
	if err != nil {
		s.Fatal("Could not get initial value of histogram: ", err)
	}

	// Crash GPUProcess. Do not crash Browser process. Crashing the Browser
	// process will disconnect our cr object.
	if dumps, err := chromecrash.KillAndGetCrashFiles(ctx, chromecrash.GPUProcess, params.crashFileType); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after logged-in Chrome crash")
	}

	// We can't use metrics.WaitForHistogramUpdate because other buckets in
	// Platform.CrOSEvent can be updated by other events in the system and we
	// don't want to stop waiting because of those updates.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		newHistogram, err := metrics.GetHistogram(ctx, cr, crashReporterHistogramName)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get new value of histogram"))
		}
		diff, err := newHistogram.Diff(oldHistogram)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "histogram diff error"))
		}
		foundMissed := false
		foundKernel := false
		for _, bucket := range diff.Buckets {
			switch bucket.Min {
			case missedCrashHistogramBucket:
				foundMissed = true
			case callFromKernelBucket:
				foundKernel = true
			}
			if foundMissed && foundKernel {
				break
			}
		}
		if params.expectMissing {
			if !foundMissed && !foundKernel {
				return errors.New("did not find either Crash.Chrome.MissedCrashes or Crash.Chrome.CrashesFromKernel in " + diff.String())
			}
			if !foundMissed {
				return errors.New("did not find Crash.Chrome.MissedCrashes in " + diff.String())
			}
		} else {
			if foundMissed {
				return testing.PollBreak(errors.New("got Crash.Chrome.MissedCrashes"))
			}
		}
		if !foundKernel {
			return errors.New("did not find Crash.Chrome.CrashesFromKernel in " + diff.String())
		}
		return nil
	}, nil)

	if err != nil {
		s.Error("Failed when looking for expected Histogram diffs: ", err)
	}
}
