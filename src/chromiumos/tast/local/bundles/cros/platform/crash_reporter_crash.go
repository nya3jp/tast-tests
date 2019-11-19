// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	platformCrash "chromiumos/tast/local/bundles/cros/platform/crash"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrashReporterCrash,
		Desc: "Verifies crash_reporter itself crashing is captured through anomaly detector",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-monitoring-forensics@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{platformCrash.TestCert},
	})
}

func setCorePatternCrashTest(crashTest bool) error {
	b, err := ioutil.ReadFile(platformCrash.CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s",
			platformCrash.CorePattern)
	}

	// Reset any crash test flag
	corePatternExpr := strings.TrimSpace(string(b))
	corePatternExpr = strings.Replace(corePatternExpr, " --crash_test", "", -1)

	if crashTest {
		corePatternExpr = corePatternExpr + " --crash_test"
	}

	if err := ioutil.WriteFile(platformCrash.CorePattern,
		[]byte(corePatternExpr), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s",
			platformCrash.CorePattern)
	}
	return nil
}

func CrashReporterCrash(ctx context.Context, s *testing.State) {
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer localCrash.TearDownCrashTest()

	if err := metrics.SetConsent(ctx, s.DataPath(platformCrash.TestCert), true); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	oldFiles, err := crash.GetCrashes(localCrash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	if err := setCorePatternCrashTest(true); err != nil {
		s.Fatal(err, "failed to replace core pattern")
	}
	defer setCorePatternCrashTest(false)

	// TODO(crbug.com/1011932): Investigate if this is necessary
	st, err := os.Stat(platformCrash.CrashReporterEnabledPath)
	if err != nil || !st.Mode().IsRegular() {
		s.Fatal("Crash reporter enabled file flag is not present at ", platformCrash.CrashReporterEnabledPath)
	}
	flagTime := time.Since(st.ModTime())
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		s.Fatal("Failed to get uptime: ", err)
	}
	if flagTime > time.Duration(uptimeSeconds)*time.Second {
		s.Fatal("User space crash handling was not started during last boot")
	}

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := localCrash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	// Intentionally crash metrics_daemon so that (doomed) crash_reporter will
	// be called.
	s.Log("Crashing metrics_daemon")
	cmd := testexec.CommandContext(ctx, "pkill", "-sigsegv", "metrics_daemon")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to induce an artifical crash: ", err)
	}

	s.Log("Waiting for crash_reporter failure files")
	expectedRegexes := []string{
		`crash_reporter_failure\.\d{8}\.\d{6}\.0\.meta`,
		`crash_reporter_failure\.\d{8}\.\d{6}\.0\.log`,
	}
	files, err := localCrash.WaitForCrashFiles(ctx, []string{localCrash.SystemCrashDir},
		oldFiles, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer localCrash.CleanupCrashFiles(files)
}
