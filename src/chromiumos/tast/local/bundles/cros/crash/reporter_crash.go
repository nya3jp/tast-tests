// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/sys/unix"

	commoncrash "chromiumos/tast/common/crash"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReporterCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies crash_reporter itself crashing is captured through anomaly detector",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-telemetry@google.com",
		},
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
			Pre:               crash.ChromePreWithVerboseConsent(),
			Val:               crash.RealConsent,
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "mock_consent",
			Val:  crash.MockConsent,
		}},
		Attr: []string{"group:mainline"},
	})
}

func setCorePatternCrashTest(ctx context.Context, crashTest bool) error {
	b, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s",
			commoncrash.CorePattern)
	}

	testing.ContextLogf(ctx, "Previous core pattern: %s", string(b))
	// Reset any crash test flag
	corePatternExpr := strings.TrimSpace(string(b))
	corePatternExpr = strings.Replace(corePatternExpr, " --crash_test", "", -1)

	if crashTest {
		corePatternExpr = corePatternExpr + " --crash_test"
	}

	testing.ContextLogf(ctx, "Setting core pattern to: %s", corePatternExpr)
	if err := ioutil.WriteFile(commoncrash.CorePattern,
		[]byte(corePatternExpr), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s",
			commoncrash.CorePattern)
	}
	return nil
}

func ReporterCrash(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()
	useConsent := s.Param().(crash.ConsentType)
	if useConsent == crash.RealConsent {
		opt = crash.WithConsent(s.PreValue().(*chrome.Chrome))
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := setCorePatternCrashTest(ctx, true); err != nil {
		s.Fatal(err, "failed to replace core pattern")
	}
	defer setCorePatternCrashTest(ctx, false)

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

	s.Log("Starting a target process")
	target := testexec.CommandContext(ctx, "/usr/bin/sleep", "300")
	if err := target.Start(); err != nil {
		s.Fatal("Failed to start a target process to kill: ", err)
	}
	defer func() {
		target.Kill()
		target.Wait()
	}()

	s.Log("Crashing the target process")
	if err := unix.Kill(target.Process.Pid, unix.SIGSEGV); err != nil {
		s.Fatal("Failed to induce an artifical crash: ", err)
	}

	s.Log("Waiting for crash_reporter failure files")
	expectedRegexes := []string{`crash_reporter_failure\.\d{8}\.\d{6}\.\d+\.0\.meta`,
		`crash_reporter_failure\.\d{8}\.\d{6}\.\d+\.0\.log`}

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	if useConsent == crash.MockConsent {
		// We might not be logged in, so also allow system crash dir.
		crashDirs = append(crashDirs, crash.SystemCrashDir)
	}

	files, err := crash.WaitForCrashFiles(ctx, crashDirs, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
