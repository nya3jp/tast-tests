// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hammerd

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TriggeredOnBoot,
		Desc:     "Hammerd smoke test to check if Hammerd is triggered on boot",
		Contacts: []string{"fshao@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  1 * time.Minute,
	})
}

func getBaseTime(s *testing.State) time.Time {
	// Get the timestamp of the current boot from the kernel log

	const kernelLogFile = "/var/log/messages"
	content, err := ioutil.ReadFile(kernelLogFile)
	if err != nil {
		s.Fatal("Failed to read kernel message: ", err)
	}

	// The pattern of the kernel boot message
	bootStartReg := regexp.MustCompile(`([-+.:\dTtZz]{20,}) INFO kernel:.*?Booting Linux`)
	matches := bootStartReg.FindAllSubmatch(content, -1)

	// We want the latest boot record
	baseTime, _ := time.Parse(time.RFC3339, string(matches[len(matches)-1][1]))
	if err != nil {
		s.Fatal("Failed to parse time: ", err)
	}

	return baseTime
}

func getHammerdLog(s *testing.State, base time.Time) []byte {
	// Extract the corresponding hammerd log based on the presumed system boot time

	const hammerdLogFile = "/var/log/hammerd.log"
	content, err := ioutil.ReadFile(hammerdLogFile)
	if err != nil {
		s.Fatal("Failed to read hammerd log: ", err)
	}

	timestampReg := regexp.MustCompile(`[-+.:\dTtZz]{20,}`)
	matches := timestampReg.FindAllSubmatchIndex(content, -1)

	// Go through every timestamps to find the first one that happened after
	// system booting
	for _, loc := range matches {
		t, _ := time.Parse(time.RFC3339, string(content[loc[0]:loc[1]]))
		if err != nil {
			s.Fatal("Failed to parse time: ", err)
		}

		if base.Before(t) {
			return content[loc[0]:]
		}
	}

	s.Fatal("No available Hammerd log after system boot")
	return nil
}

func checkLog(s *testing.State, log []byte) {
	// Go through the extracted log and do the preliminary checks

	findLog := func(pat string) [][]byte {
		reg := regexp.MustCompile(pat)
		return reg.FindSubmatch(log)
	}

	// Message that is printed when base is not connected on boot
	// It's defined at `src/platform2/hammerd/init/hammerd-at-boot.sh`
	if findLog(`Base not connected, skipping hammerd at boot`) != nil {
		s.Fatal("Hammerd was not triggered on boot")
	}

	// Message that is printed when hammerd upstart job exited abnormally
	if match := findLog(`hammerd main process \([0-9]+\) terminated with value ([0-9]+)`); match != nil {
		s.Fatal("Uammerd terminated with non-zero value: ", string(match[1]))
	}

	// Hammerd writes the USB device path to the base into this file
	const devicePathFile = "/run/metrics/external/hammer/hammer_sysfs_path"
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

func TriggeredOnBoot(ctx context.Context, s *testing.State) {
	baseTime := getBaseTime(s)
	s.Log("Presumed system boot time: ", baseTime)

	log := getHammerdLog(s, baseTime)
	checkLog(s, log)
}
