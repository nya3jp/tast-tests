// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cgroups

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/testing"
)

// TestCPUSet verifies the correct CPU pinning for ARC
func TestCPUSet(ctx context.Context, s *testing.State, a *arc.ARC) {
	s.Log("Running testCPUSet")

	SDKVer, err := arc.SDKVersion()
	if err != nil {
		s.Error("Failed to find SDKVersion: ", err)
		return
	}

	CPUSpec := map[string]func(cpuset.CPUSet) bool{
		"foreground":        cpuset.Online().Equal,
		"top-app":           cpuset.Online().Equal,
		"background":        cpuset.Online().StrictSuperset,
		"system-background": cpuset.Online().StrictSuperset,
	}
	if SDKVer >= arc.SDKP {
		// In ARC P or later, restricted is added.
		CPUSpec["restricted"] = cpuset.Online().StrictSuperset
	}

	cpuset.CheckCPUSpec(s, CPUSpec)
}
