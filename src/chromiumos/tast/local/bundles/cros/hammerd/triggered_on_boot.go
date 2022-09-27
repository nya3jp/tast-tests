// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TriggeredOnBoot,
		Desc:         "Hammerd smoke test to check if Hammerd is triggered on boot",
		Contacts:     []string{"fshao@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"hammerd"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.ECFeatureDetachableBase()),
		Timeout:      1 * time.Minute,
	})
}

func TriggeredOnBoot(ctx context.Context, s *testing.State) {
	const LogRotateLimit = 7
	var logs []string
	var logFound = false

	// The system rotates the logs on a daily basis (i.e. renaming xxx.log to
	// xxx.1.log and so on) and keeps them for a week at most. In other words,
	// it's possible that the full log is split into different log files.
	// To address that, We need to start from the eldest log and concatenate them
	// together.
	//
	// Note that the syslog package is not used here because it does not provide
	// enough support to achieve what we need here, and people prefer to keep
	// the syslog reader simple and general to support all kinds of syslogs.
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
		s.Fatal("Failed to locate hammerd-at-boot log. ",
			"The system may have failed to invoke Hammerd upstart job")
	}

	// Look for patterns that indicate Hammerd failures.
	// Note that hwdep ensures the base is currently attached, but that doesn't
	// mean the base was also attached when the system booted.
	for _, line := range logs {
		if strings.Contains(line, "Base not connected, skipping hammerd at boot") {
			s.Fatal("Base is currently attached, but Hammerd didn't find it at boot")
		}
		if strings.Contains(line, "Send the DBus signal: BaseFirmwareUpdateFailed") {
			s.Fatal("Hammerd failed to update base firmware at boot")
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
		s.Fatal("Autosuspend is not enabled on the detachable base: ", control)
	}
}
