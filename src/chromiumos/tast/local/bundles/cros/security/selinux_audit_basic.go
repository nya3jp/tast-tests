// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxAuditBasic,
		Desc:         "Checks SELinux audit works as intended",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux"},
		// TODO(b/245411884): Re-enable this test.
		// Attr:         []string{"group:mainline"},
	})
}

func SELinuxAuditBasic(ctx context.Context, s *testing.State) {
	// Directory name should keep in sync with platform2/sepolicy/policy/chromeos/dev/cros_ssh_session
	const markerDirName = "cros_selinux_audit_basic_test"

	s.Log("Waiting for auditd job to be running")
	if err := upstart.WaitForJobStatus(ctx, "auditd", upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("Failed waiting for auditd to start: ", err)
	}

	hasLineMatch := func(r io.Reader, re *regexp.Regexp) (bool, error) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if re.MatchString(line) {
				return true, nil
			}
		}
		return false, scanner.Err()
	}

	// Generate an audit event by creating a file inside markerDirectory
	td, err := ioutil.TempDir("/tmp", "tast.security.SELinuxAuditBasic.")
	if err != nil {
		s.Fatal("Failed to create temporary directory for testing: ", err)
	}
	defer os.RemoveAll(td)
	markerDirectory := filepath.Join(td, markerDirName)
	if err := os.Mkdir(markerDirectory, 0700); err != nil {
		s.Fatal("Failed to create marker directory for testing: ", err)
	}
	f, err := ioutil.TempFile(markerDirectory, "audit-marker-")
	if err != nil {
		s.Fatal("Failed to create marker file: ", err)
	}
	fileName := path.Base(f.Name())
	f.Close()

	// Checks log can be found in audit.log for file name.
	// TODO(yoshiki): Replace this with croslog command.
	f, err = os.Open("/var/log/audit/audit.log")
	if err != nil {
		s.Fatal("Failed to open audit.log: ", err)
	}
	defer f.Close()

	// Try reading multiple times, since there is a possibility of delay in
	// auditd's wriging to the log file.
	const (
		retryTimeout  = 10 * time.Second
		retryInterval = 1 * time.Second
	)
	wantedLine, err := regexp.Compile("granted.*" + fileName)
	if err != nil {
		s.Fatal("Regexp compile error. The path of temporary file may be wrong: ", err)
	}
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if match, err := hasLineMatch(f, wantedLine); err != nil {
			// Failed: something is wrong in reading the log file.
			return testing.PollBreak(errors.Wrap(err, "failed to read audit.log"))
		} else if !match {
			// Retry after sleep.
			return errors.New("expected audit message is not found in audit.log")
		}
		// Succeeded: the log entry is found.
		return nil
	}, &testing.PollOptions{Timeout: retryTimeout, Interval: retryInterval}); err != nil {
		// Failed: the retry count exceeded.
		s.Error("Expected audit message in audit.log but not found: ", err)
	}
}
