// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

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
	// Examine auditd is running.
	_, state, _, err := upstart.JobStatus(ctx, "auditd")
	if err != nil {
		s.Error("Failed to get upstart job status: ", err)
	} else if state != "running" {
		s.Error("auditd is not runing: ", state)
	}

	// Examine /var/log/messages to make sure no audit message after 10s.
	out, err := exec.Command("/usr/bin/journalctl", "-b", "0", "-o", "short-monotonic", "-t", "kernel", "--grep", "audit: type=").Output()
	badLineRegexp := regexp.MustCompile(`.*[0-9]{2}\.[0-9]*\] (kernel: )?audit: type=.*`)
	if err != nil {
		s.Error("Failed to read syslog: ", err)
	} else if badLine := badLineRegexp.FindString(string(out)); badLine != "" {
		s.Errorf("Found bad audit line in syslog: %q", badLine)
	}

	// Examine /var/log/audit/
	auditFilesPath, err := filepath.Glob("/var/log/audit/audit.log*")
	emptyAuditLog := true
	if err != nil {
		s.Error("Failed to locate audit log file /var/log/audit/audit.log*")
	} else {
		for _, auditFilePath := range auditFilesPath {
			fi, err := os.Stat(auditFilePath)
			if err != nil {
				s.Error("Fail to stat file: ", auditFilePath, err)
			}
			if fi.Size() > 0 {
				emptyAuditLog = false
			}
		}
		if emptyAuditLog {
			s.Error("audit log is empty")
		}
	}

	// Testing auditd is working at this present.
	// TODO(fqj) we need to add some policy to intentially audit something.
}
