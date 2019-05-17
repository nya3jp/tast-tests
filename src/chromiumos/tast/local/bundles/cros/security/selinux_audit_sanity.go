// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxAuditSanity,
		Desc:         "Checks SELinux audit works as intended",
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxAuditSanity(ctx context.Context, s *testing.State) {
	// Directory name should keep in sync with platform2/sepolicy/policy/chromeos/dev/cros_ssh_session
	const markerDirectory = "/tmp/cros_selinux_audit_sanity_test"

	s.Log("Waiting for auditd job to be running")
	if err := upstart.WaitForJobStatus(ctx, "auditd", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("Failed waiting for auditd to start: ", err)
	}

	randStr := func(l int) string {
		var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		s := make([]rune, l)
		for i := range s {
			s[i] = letters[rand.Intn(len(letters))]
		}
		return string(s)
	}

	findFirstMatchedLine := func(r io.Reader, re *regexp.Regexp) (string, error) {
		rr := bufio.NewReader(r)
		for {
			line, err := rr.ReadString('\n')
			if err == io.EOF {
				return "", nil
			}
			if err != nil {
				return "", err
			}
			if re.MatchString(line) {
				return line, nil
			}
		}
	}

	// Generate an audit event by creating a file inside markerDirectory
	os.Mkdir(markerDirectory, 0755)
	fileName := randStr(20)
	fi, err := os.Create(filepath.Join(markerDirectory, fileName))
	if err != nil {
		s.Fatal("Failed to create marker file: ", err)
	}
	fi.Close()
	defer os.RemoveAll(markerDirectory)

	// Checks no logs matching the file name in syslog.
	badContent, err := exec.Command("journalctl", "-b", "0", "-t", "kernel", "--grep", fileName).Output()
	if err != nil {
		s.Fatal("Failed to read syslog from journald: ", err)
	}
	// journalctl adds a '-- No Entries --' line if --grep is not found.
	// We need to check the output actually contains the file name.
	if strings.Contains(string(badContent), fileName) {
		s.Errorf("audit shouldn't be logged to syslog, but found: %q", badContent)
	}

	// Checks log can be found in audit.log for file name.
	f, err := os.Open("/var/log/audit/audit.log")
	if err != nil {
		s.Fatal("Failed to open audit.log")
	}
	defer f.Close()
	wantedLine := regexp.MustCompile("granted.*" + fileName)
	foundLine, err := findFirstMatchedLine(f, wantedLine)
	if err != nil {
		s.Fatal("Failed to read audit.log: ", err)
	}
	if foundLine == "" {
		s.Error("Expected audit message in audit.log but not found")
	}
}
