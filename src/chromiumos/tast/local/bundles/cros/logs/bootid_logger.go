// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

import (
	"context"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootidLogger,
		Desc:         "Tests related with bootid-logger",
		Contacts:     []string{"yoshiki@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func BootidLogger(ctx context.Context, s *testing.State) {
	const (
		bootidLoggerExecutable = "/usr/sbin/bootid-logger"
	)

	s.Log("Running bootid-logger")
	out, err := testexec.CommandContext(ctx, bootidLoggerExecutable).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("bootid-logger fails with output %q: %v", out, err)
	}

	bootID, err := getCurrentBootID()
	if err != nil {
		s.Error("Failed to retrieve the current boot id: ", err)
	}

	bootIDLog, err := getCurrentBootIDLog()
	if err != nil {
		s.Error("Failed to retrieve the content of the boot id log: ", err)
	}

	lines := strings.Split(strings.TrimSpace(bootIDLog), "\n")
	if len(lines) == 0 {
		s.Error("Failed to retrieve the content of the boot id log. The log contains no line: ")
	}

	lastLine := lines[len(lines)-1]

	if !strings.HasSuffix(lastLine, bootID) {
		s.Error("The last entry of the boot id log doesn't contain the current boot id: ", err)
	}
}

func getCurrentBootID() (string, error) {
	out, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")

	if err != nil {
		return "", errors.Wrap(err, "failed to read the current boot id")
	}
	return strings.ReplaceAll(strings.TrimSpace(string(out)), "-", ""), nil
}

func getCurrentBootIDLog() (string, error) {
	out, err := ioutil.ReadFile("/var/log/boot_id.log")

	if err != nil {
		return "", errors.Wrap(err, "failed to read the content of the boot id log")
	}
	return strings.TrimSpace(string(out)), nil
}
