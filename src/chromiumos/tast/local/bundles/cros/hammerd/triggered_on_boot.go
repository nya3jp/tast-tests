// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hammerd

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TriggeredOnBoot,
		Desc:         "Hammerd smoke test to check if Hammerd is triggered on boot",
		Contacts:     []string{"fshao@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"hammerd"},
		Timeout:      1 * time.Minute,
	})
}

func TriggeredOnBoot(ctx context.Context, s *testing.State) {
	const LogRotateLimit = 7
	var logs []string
	var logFound = false

	// Getting the full hammerd log for the current boot while considering that
	// the log may have been rotated e.g. hammerd.log => hammerd.1.log.
	//
	// Note that the syslog package is not used here because it does not provide
	// enough support to achieve what we need here, and people prefer to keep
	// the syslog reader simple unless there's a good solution that can be
	// applied to all kinds of syslogs.
	//
	// Duplicating the full log of the current boot into one single file may help
	// in this case, but unfortunately rsyslog doesn't seem to have the
	// permission to write log to /tmp. (maybe tmpfs was not yet mounted by the
	// time write operations were issued)
	for i := LogRotateLimit; i >= 0; i-- {
		var logPath string
		if i > 0 {
			logPath = fmt.Sprintf("/var/log/hammerd.%d.log", i)
		} else {
			logPath = "/var/log/hammerd.log"
		}

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			// The log doesn't exist, but it's fine.
			continue
		}
		file, err := os.Open(logPath)
		if err != nil {
			s.Fatalf("Failed to open log %s: %v", logPath, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// The line defined in `src/platform2/hammerd/init/hammerd-at-boot.sh`
			// indicates the upstart job invocation, which means a new log iteration
			// is found, so we drop the previous log.
			if strings.Contains(line, "Start checking base status") {
				logs = []string{line}
				logFound = true
			} else {
				logs = append(logs, line)
			}
		}
		if err := scanner.Err(); err != nil {
			s.Fatal("Scanner error: ", err)
		}
	}
	if !logFound {
		s.Fatal("Cannot find available hammerd log to validate")
	}

	// Look for patterns that indicate Hammerd failures.
	for _, line := range logs {
		if strings.Contains(line, "Base not connected, skipping hammerd at boot") {
			s.Fatal("Base not connected, Hammerd was not triggered on boot")
		}
		if strings.Contains(line, "Send the DBus signal: BaseFirmwareUpdateFailed") {
			s.Fatal("Hammerd failed to update base firmware")
		}
	}

	// Hammerd writes the USB device path to the base into this file.
	devicePathFile := "/run/metrics/external/hammer/hammer_sysfs_path"
	buf, err := ioutil.ReadFile(devicePathFile)
	if err != nil {
		s.Fatal("Failed to read the device path: ", devicePathFile)
	}
	sysfsPath := strings.TrimSuffix(string(buf), "\n")

	// Check if autosuspend is enabled.
	powerControlFile := filepath.Join(sysfsPath, "power/control")
	buf, err = ioutil.ReadFile(powerControlFile)
	if err != nil {
		s.Fatal("Failed to read the power control: ", powerControlFile)
	} else if control := strings.TrimSuffix(string(buf), "\n"); control != "auto" {
		s.Fatal("Autosuspend is not enabled on hammer port: ", control)
	}
}
