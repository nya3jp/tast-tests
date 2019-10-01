// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	platform_crash "chromiumos/tast/local/bundles/cros/platform/crash"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/host"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrashReporterCrash,
		Desc: "Verifies crash_reporter itself crashing is captured through anomaly detector.",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-monitoring-forensics@google.com",
		},
		Attr: []string{"informational"},
		Data: []string{platform_crash.TestCert},
	})
}

func replaceCorePattern(crash_test bool) error {
	corePatternExpr := fmt.Sprintf("|%s --user=%%P:%%s:%%u:%%g:%%e", platform_crash.CrashReporterPath)

	if crash_test {
		corePatternExpr = corePatternExpr + " --crash_test"
	}

	if err := ioutil.WriteFile(platform_crash.CorePattern, []byte(corePatternExpr), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", platform_crash.CorePattern)
	}
	return nil
}

func CrashReporterCrash(ctx context.Context, s *testing.State) {
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer localCrash.TearDownCrashTest()

	if err := metrics.SetConsent(ctx, s.DataPath(platform_crash.TestCert), true); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	oldFiles, err := crash.GetCrashes(localCrash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	if err := replaceCorePattern(true); err != nil {
		errors.Wrap(err, "failed to replace core pattern")
	}

	f, err := os.Stat(crashReporterEnabledPath)
	if err != nil || !f.Mode().IsRegular() {
		s.Error("Crash reporter enabled file flag is not present at ", crashReporterEnabledPath)
		return
	}
	flagTime := time.Since(f.ModTime())
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		s.Error("Failed to get uptime: ", err)
		return
	}
	if flagTime > time.Duration(uptimeSeconds)*time.Second {
		s.Error("User space crash handling was not started during last boot")
	}

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := localCrash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	// Induce any kind of crash so that crash_reporter will be called which in turn will also crash.
	s.Log("Inducing any ol crash")
	if err := testexec.CommandContext(ctx, "kill", "-s", "SIGSEGV", "$$"); err != nil {
		s.Fatal("Could not induce a crash", err)
	}

	s.Log("Waiting for crash files")
	expectedRegexes := []string{`crash_reporter_failure\.\d{8}\.\d{6}\.0\.meta`}
	files, err := localCrash.WaitForCrashFiles(ctx, localCrash.SystemCrashDir, oldFiles, expectedRegexes)
	if err != nil {
		s.Error("Couldn't find expected files: ", err)
	}
	// Clean up files.
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			s.Logf("Couldn't clean up %s: %v", f, err)
		}
	}
}
