// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
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
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxAuditSanity(ctx context.Context, s *testing.State) {
	// wait for auditd.
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

	// Directory name should keep in sync with platform2/sepolicy/policy/chromeos/dev/cros_ssh_session
	os.Mkdir("/tmp/cros_selinux_audit_sanity_test", 0755)
	fileName := randStr(20)
	fi, err := os.Create(filepath.Join("/tmp/cros_selinux_audit_sanity_test", fileName))
	if err != nil {
		s.Fatal("Failed to create marker file: ", err)
	}
	fi.Close()

	content, err := exec.Command("journalctl", "-b", "0", "-t", "kernel", "--grep", fileName).Output()
	badLineRegexp := regexp.MustCompile(".*" + fileName + ".*")
	if err != nil {
		s.Fatal("Failed to read syslog from journald: ", err)
	}
	if badLineRegexp.MatchString(string(content)) {
		s.Errorf("audit shouldn't be logged to syslog, but found: %q", content)
	}

	content, err = exec.Command("egrep", "granted.*"+fileName, "/var/log/audit/audit.log").Output()
	// grep will return non-zero status 1 if no match, status 2 is file not found.
	if err != nil {
		s.Fatal("Failed to locate expected audit message in audit.log: ", err)
	}
}
