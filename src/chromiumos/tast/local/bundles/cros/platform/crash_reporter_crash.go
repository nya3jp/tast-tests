// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/host"
	"golang.org/x/sys/unix"

	commoncrash "chromiumos/tast/common/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
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
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func setCorePatternCrashTest(crashTest bool) error {
	b, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s",
			commoncrash.CorePattern)
	}

	// Reset any crash test flag
	corePatternExpr := strings.TrimSpace(string(b))
	corePatternExpr = strings.Replace(corePatternExpr, " --crash_test", "", -1)

	if crashTest {
		corePatternExpr = corePatternExpr + " --crash_test"
	}

	if err := ioutil.WriteFile(commoncrash.CorePattern,
		[]byte(corePatternExpr), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s",
			commoncrash.CorePattern)
	}
	return nil
}

func CrashReporterCrash(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	if err := setCorePatternCrashTest(true); err != nil {
		s.Fatal(err, "failed to replace core pattern")
	}
	defer setCorePatternCrashTest(false)

	// TODO(crbug.com/1011932): Investigate if this is necessary
	st, err := os.Stat(commoncrash.CrashReporterEnabledPath)
	if err != nil || !st.Mode().IsRegular() {
		s.Fatal("Crash reporter enabled file flag is not present at ", commoncrash.CrashReporterEnabledPath)
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
	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	s.Log("Starting a dummy process")
	dummy := testexec.CommandContext(ctx, "/usr/bin/sleep", "1m")
	if err := dummy.Start(); err != nil {
		s.Fatal("Failed to start a dummy process to kill: ", err)
	}
	defer func() {
		dummy.Kill()
		dummy.Wait()
	}()

	s.Log("Crashing the dummy process")
	if err := unix.Kill(dummy.Process.Pid, syscall.SIGSEGV); err != nil {
		s.Fatal("Failed to induce an artifical crash: ", err)
	}

	s.Log("Waiting for crash_reporter failure files")
	expectedRegexes := []string{`crash_reporter_failure\.\d{8}\.\d{6}\.0\.meta`,
		`crash_reporter_failure\.\d{8}\.\d{6}\.0\.log`}

	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir},
		oldFiles, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
