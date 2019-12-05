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

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashReporterMissMetrics,
		Desc:     "Checks that anomaly detector reports when crash_reporter was not invoked",
		Contacts: []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ChromeCrashReporterMissMetrics(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	// Make sure anomaly_detector isn't running, and then clear pending metrics.
	// This ensures there's no events from before the test starts.
	if err := upstart.StopJob(ctx, "anomaly-detector"); err != nil {
		s.Error("Upstart couldn't stop anomaly-detector: ", err)
	}
	// See ClearHistogramTransferFile for more about why this is needed.
	if err := metrics.ClearHistogramTransferFile(); err != nil {
		s.Error("Could not truncate existing metrics files: ", err)
	}

	// By not adding the "chrome.CrashNormalMode()" option, we rely on the crash
	// handling that bypasses the crash_reporter, thus anomaly_detector should
	// count the crash as a missed crash. (See code in chrome.restartChromeForTesting,
	// where it sets CHROME_HEADLESS and BREAKPAD_DUMP_LOCATION environmental variables)
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	if err = crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Could not restart anomaly detector: ", err)
	}
	oldHistogram, err := metrics.GetHistogram(ctx, cr, chromecrash.CrashReporterHistogramName)
	if err != nil {
		s.Fatal("Could not get initial value of histogram: ", err)
	}

	// Crash GPUProcess. Do not crash Browser process. Crashing the Browser
	// process will disconnect our cr object.
	if dumps, err := chromecrash.KillAndGetCrashFiles(ctx, chromecrash.GPUProcess, chromecrash.BreakpadDmp); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after logged-in Chrome crash")
	}

	// We can't use metrics.WaitForHistogramUpdate because other buckets in
	// Platform.CrOSEvent can be updated by other events in the system and we
	// don't want to stop waiting because of those updates.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		newHistogram, err := metrics.GetHistogram(ctx, cr, chromecrash.CrashReporterHistogramName)
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
			case chromecrash.MissedCrashHistogramBucket:
				foundMissed = true
			case chromecrash.CallFromKernelBucket:
				foundKernel = true
			}
			if foundMissed && foundKernel {
				return nil
			}
		}
		if !foundMissed && !foundKernel {
			return errors.New("Did not find either Crash.Chrome.MissedCrashes or Crash.Chrome.CrashesFromKernel in " + diff.String())
		}
		if !foundMissed {
			return errors.New("Did not find Crash.Chrome.MissedCrashes in " + diff.String())
		}
		return errors.New("Did not find Crash.Chrome.CrashesFromKernel in " + diff.String())
	}, nil)

	if err != nil {
		s.Error("Failed when looking for expected Histogram diffs: ", err)
	}
}
