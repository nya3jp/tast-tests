// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckTracefsInstances,
		Desc:     "Checks that the number of tracefs instances doesn't exceed the number of CPU cores/threads",
		Contacts: []string{"chinglinyu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// CheckTracefsInstances checks that we don't have more tracefs instances than the number of processors as a warning of potential boot performance issues with SELinux labeling.
func CheckTracefsInstances(ctxtx context.Context, s *testing.State) {
	const tracefsInstancesPath = "/sys/kernel/debug/tracing/instances"
	// Regex to match "processor : 1" in /proc/cpuinfo.
	processorRE := regexp.MustCompile(`^processor\s+:\s+\d+$`)

	cpuinfo, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		s.Error("Failed to read from /proc/cpuinfo: ", err)
	}

	lines := strings.Split(string(cpuinfo), "\n")
	processorCount := 0
	for _, line := range lines {
		matches := processorRE.FindAllStringSubmatch(strings.TrimSpace(line), -1)
		if matches != nil {
			processorCount++
		}
	}
	if processorCount == 0 {
		s.Error("Failed to get the processor count from /proc/cpuinfo")
	}

	_, err = os.Stat(tracefsInstancesPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Error("Failed to check the status of tracefs instances: ", err)
	}

	instances, err := ioutil.ReadDir(tracefsInstancesPath)
	if err != nil {
		s.Error("Failed to read the instances directory: ", err)
	}

	instancesCount := 1 // The default tracefs instance.
	for _, instance := range instances {
		if instance.IsDir() {
			instancesCount++ // Found additional instances.
		}
	}

	s.Logf("Processor count: %d, tracefs instances: %d", processorCount, instancesCount)
	if instancesCount > 0 && instancesCount > processorCount {
		// This is potentially harmful to boot performance. Fail the test as a warning.
		s.Errorf("More tracefs instances than processor count (%d > %d): tracefs labeling is potentially slow", instancesCount+1, processorCount)
	}
}
