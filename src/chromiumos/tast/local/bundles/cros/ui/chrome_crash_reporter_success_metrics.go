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
	consent "chromiumos/tast/local/metrics"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashReporterSuccessMetrics,
		Desc:     "Checks that anomaly detector reports when crash_reporter was invoked",
		Contacts: []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{chromecrash.TestCert},
	})
}

func ChromeCrashReporterSuccessMetrics(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	if err := consent.SetConsent(ctx, s.DataPath(chromecrash.TestCert), true); err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	// Make sure anomaly_detector isn't running, and then clear pending metrics.
	// This ensures there's no events from before the test starts. We only
	// warn on errors since failing to stop anomaly-detector may or may not mess
	// up the test.
	if err := upstart.StopJob(ctx, "anomaly-detector"); err != nil {
		s.Log("Upstart couldn't stop anomaly-detector: ", err)
	}
	if err := metrics.ClearHistogramTransferFile(); err != nil {
		s.Log("Could not truncate existing metrics files: ", err)
	}

	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(chromecrash.VModuleFlag))
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
	if dumps, err := chromecrash.KillAndGetCrashFiles(ctx, chromecrash.GPUProcess, chromecrash.MetaFile); err != nil {
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
		foundKernel := false
		for _, bucket := range diff.Buckets {
			switch bucket.Min {
			case chromecrash.MissedCrashHistogramBucket:
				return testing.PollBreak(errors.New("got Crash.Chrome.MissedCrashes"))
			case chromecrash.CallFromKernelBucket:
				foundKernel = true
				// Don't break out of loop or return nil, but keep looking for
				// events in the MissedCrashHistogramBucket.
			}
		}
		if !foundKernel {
			return errors.New("did not find Crash.Chrome.CrashesFromKernel in " + diff.String())
		}
		return nil
	}, nil)

	if err != nil {
		s.Error("Anomaly detector test failed: ", err)
	}
}
