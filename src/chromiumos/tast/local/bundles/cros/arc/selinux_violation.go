// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelinuxViolation,
		Desc:         "Test handling of an ARC++ selinux violation",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome", "selinux"},
	})
}

func SelinuxViolation(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	// Ignore non-selinux violations for the duration of the test.
	crash.EnableCrashFiltering(ctx, "selinux")
	defer crash.DisableCrashFiltering()

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	// Make sure that auditd is running since anomaly-detector reads audit.log written by auditd to monitor selinux violations. crbug.com/1113078
	if err := upstart.CheckJob(ctx, "auditd"); err != nil {
		s.Fatal("Auditd is not running: ", err)
	}

	s.Log("Generating audit event by starting arc++")

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to prepare syslog reader in RunCrasherProcess: ", err)
	}
	defer reader.Close()

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	s.Log("Waiting for anomaly_detector message")
	// Wait until anomaly detector indicates that it saw, but is skipping, the violation.
	waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := reader.Wait(waitCtx, time.Hour /* unused */, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "Skipping non-CrOS selinux violation")
	})
	if err != nil {
		s.Error("logs did not contain anomaly-detector's 'skipping' message")
	}

	base := fmt.Sprintf(`selinux_violation_\S+\.\d{8}\.\d{6}\.\d+\.\d+`)
	logFileRegex := base + `.log`
	metaFileRegex := base + `.meta`
	expectedRegexes := []string{logFileRegex, metaFileRegex}

	s.Log("Waiting for crash files")
	// Only wait 5 seconds since we already saw the anomaly_detector
	// skipping message and don't (necessarily) expect there to be any
	// crashes.
	waitCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	// Wait for selinux files. These may include violations anomaly_detector
	// *should* process (those whose contexts or comms contain "cros" or "minijail").
	files, err := crash.WaitForCrashFiles(waitCtx, []string{crash.SystemCrashDir}, expectedRegexes)

	if err == nil {
		defer crash.RemoveAllFiles(ctx, files)
		// verify that all log files contain `cros` or `minijail`.
		// anomaly_detector should ignore those violations that do not
		// contain these strings.
		for _, f := range files[logFileRegex] {
			contents, err := ioutil.ReadFile(f)
			if err != nil {
				s.Errorf("Couldn't read log file %s: %v", f, err)
				continue
			}
			if !strings.Contains(string(contents), "cros") && !strings.Contains(string(contents), "minijail") {
				s.Errorf("Bad contents %s in %s. Saving file", string(contents), f)
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), f); err != nil {
					s.Error("Could not move file to out dir: ", err)
				}
			}
		}
	}

}
