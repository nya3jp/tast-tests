// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"

	"github.com/shirou/gopsutil/v3/cpu"

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
	np := len(cpuInfo) // np: number of processors.

	instances, err := ioutil.ReadDir(tracefsInstancesPath)
	if err != nil {
		s.Fatal("Failed to read the instances directory: ", err)
	}

	ni := 1 // ni: number of instances, starting from 1 (the default instance).
	for _, instance := range instances {
		if instance.IsDir() {
			ni++ // Found additional instances.
		}
	}

	s.Logf("Number of processors: %d, number of tracefs instances: %d", np, ni)
	if ni > 0 && ni > np {
		// This is potentially harmful to boot performance. Fail the test as a warning.
		s.Fatalf("More tracefs instances than the number of processors (got %d, want <= %d): tracefs labeling is potentially slow", ni, np)
	}
}
