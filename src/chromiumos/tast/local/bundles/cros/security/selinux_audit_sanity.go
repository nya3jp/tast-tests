// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"github.com/shirou/gopsutil/process"
	"os"
	"path/filepath"
	"regexp"

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

func readFileLines(path string) (result []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	return result, scanner.Err()
}

func SELinuxAuditSanity(ctx context.Context, s *testing.State) {
	// Examine auditd is running
	procs, err := process.Processes()
	if err != nil {
		s.Error("Failed to get process listing: ", err)
	}
	foundAuditd := false
	for _, proc := range procs {
		if name, err := proc.Name(); err == nil && name == "auditd" {
			foundAuditd = true
		}
	}
	if !foundAuditd {
		s.Error("auditd is not runing")
	}

	// Examine /var/log/messages to make sure no audit message after 60s.
	syslogFilesPath, err := filepath.Glob("/var/log/messages*")
	if err != nil {
		s.Fatal("Failed to locate syslog file /var/log/messages*")
	}
	badLineRegexp := regexp.MustCompile(`.*[0-9]{2}\.[0-9]*\] audit: type=.*`)
	for _, syslogFilePath := range syslogFilesPath {
		lines, err := readFileLines(syslogFilePath)
		if err != nil {
			s.Errorf("Failed to read file: %q", syslogFilesPath)
			continue
		}
		for _, line := range lines {
			if badLineRegexp.MatchString(line) {
				s.Errorf("Found bad audit line in syslog: %q", line)
			}
		}
	}

	// Examine /var/log/audit/
	auditFilesPath, err := filepath.Glob("/var/log/audit/audit.log*")
	if err != nil {
		s.Fatal("Failed to locate audit log file /var/log/audit/audit.log*")
	}
	nlines := 0
	for _, auditFilePath := range auditFilesPath {
		lines, err := readFileLines(auditFilePath)
		if err != nil {
			s.Errorf("Failed to read file: %q", auditFilePath)
			continue
		}
		nlines += len(lines)
	}
	if nlines <= 0 {
		s.Error("audit log is empty")
	}

	// Testing auditd is working at this present
	// TODO(fqj) we need to add some policy to intentially audit something.
}
