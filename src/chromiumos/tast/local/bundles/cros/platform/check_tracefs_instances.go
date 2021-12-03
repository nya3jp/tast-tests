// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"

	"github.com/shirou/gopsutil/cpu"

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
func CheckTracefsInstances(ctx context.Context, s *testing.State) {
	const tracefsInstancesPath = "/sys/kernel/debug/tracing/instances"

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err != nil {
		s.Fatal("Failed to get CPU info: ", err)
	}
	processorCount := len(cpuInfo)

	instances, err := ioutil.ReadDir(tracefsInstancesPath)
	if err != nil {
		s.Fatal("Failed to read the instances directory: ", err)
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
		s.Fatalf("More tracefs instances than processor count (%d > %d): tracefs labeling is potentially slow", instancesCount, processorCount)
	}
}
