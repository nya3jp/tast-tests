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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxAuditSanity,
		Desc:         "Checks SELinux audit works as intended",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxAuditSanity(ctx context.Context, s *testing.State) {
	// Directory name should keep in sync with platform2/sepolicy/policy/chromeos/dev/cros_ssh_session
	const markerDirName = "cros_selinux_audit_sanity_test"

	s.Log("Waiting for auditd job to be running")
	if err := upstart.WaitForJobStatus(ctx, "auditd", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
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
	td, err := ioutil.TempDir("/tmp", "tast.security.SELinuxAuditSanity.")
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

	// Checks no logs matching the file name in syslog.
	if badContent, err := exec.Command("journalctl", "-q", "-b", "0", "-t", "kernel", "--grep", fileName).Output(); err != nil {
		s.Fatal("Failed to read syslog from journald: ", err)
	} else if string(badContent) != "" {
		s.Errorf("audit shouldn't be logged to syslog, but found %q", badContent)
	}

	// Checks log can be found in audit.log for file name.
	f, err = os.Open("/var/log/audit/audit.log")
	if err != nil {
		s.Fatal("Failed to open audit.log: ", err)
	}
	defer f.Close()
	wantedLine := regexp.MustCompile("granted.*" + fileName)
	if match, err := hasLineMatch(f, wantedLine); err != nil {
		s.Fatal("Failed to read audit.log: ", err)
	} else if !match {
		s.Error("Expected audit message in audit.log but not found")
	}
}
