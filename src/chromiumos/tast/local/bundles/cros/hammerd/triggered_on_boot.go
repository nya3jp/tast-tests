// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hammerd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/syslog"
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
	var logs []*syslog.Entry
	var logFound = false

	// Helper function to match patterns in log entries
	findLog := func(pat string, e *syslog.Entry) []string {
		reg := regexp.MustCompile(pat)
		return reg.FindStringSubmatch(e.Content)
	}

	// The log may have been rotated e.g. hammerd.log => hammerd.1.log
	for i := LogRotateLimit; i >= 0; i-- {
		var logPath string
		if i > 0 {
			logPath = fmt.Sprintf("/var/log/hammerd.%d.log", i)
		} else {
			logPath = "/var/log/hammerd.log"
		}

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			continue
		}

		reader, err := syslog.NewReader(ctx, syslog.SourcePath(logPath), syslog.ReadFromStart(true))
		if err != nil {
			s.Fatalf("Failed to get reader for %s: %v", logPath, err)
		}
		defer reader.Close()

		// We only want the log belongs to the latest boot cycle, so we look for
		// the line that is always printed right after the hammerd upstart job is
		// invoked.
		for {
			line, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				s.Fatal("Failed to read log: ", err)
			}

			// The line defined in `src/platform2/hammerd/init/hammerd-at-boot.sh`
			// indicates the upstart job invocation, which means a new log iteration
			// is found, so we drop the previous log.
			if findLog(`Start checking base status`, line) != nil {
				logs = []*syslog.Entry{line}
				logFound = true
			} else {
				logs = append(logs, line)
			}
		}
	}

	if logFound == false {
		s.Fatal("Cannot find available hammerd log to validate")
	}

	// Look for patterns that indicate Hammerd failures
	for _, line := range logs {
		if findLog(`Base not connected, skipping hammerd at boot`, line) != nil {
			s.Fatal("Base not connected, Hammerd was not triggered on boot")
		}
		if findLog(`Send the DBus signal: BaseFirmwareUpdateFailed`, line) != nil {
			s.Fatal("Hammerd failed to update base firmware")
		}
	}

	// Hammerd writes the USB device path to the base into this file
	devicePathFile := "/run/metrics/external/hammer/hammer_sysfs_path"
	buf, err := ioutil.ReadFile(devicePathFile)
	if err != nil {
		s.Fatal("Failed to read the device path: ", devicePathFile)
	}
	sysfsPath := strings.TrimSuffix(string(buf), "\n")

	// Check if autosuspend is enabled
	powerControlFile := filepath.Join(sysfsPath, "power/control")
	buf, err = ioutil.ReadFile(powerControlFile)
	if err != nil {
		s.Fatal("Failed to read the power control: ", powerControlFile)
	} else if control := strings.TrimSuffix(string(buf), "\n"); control != "auto" {
		s.Fatal("Autosuspend is not enabled on hammer port: ", control)
	}
}
