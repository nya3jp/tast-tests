// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package qemu

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Validity,
		Desc: "Set baseline expectations for hosting Chrome OS VM images",
		Contacts: []string{
			"pwang@chromium.org", // Original test author
			"cros-containers-dev@google.com",
			"oka@chromium.org", // Tast port author
		},
		// This test should be kept as informational, because if not informational, as it detects a change in GCE
		// environment, any change to GCE could break the CQ.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"qemu"},
	})
}

func Validity(ctx context.Context, s *testing.State) {
	// Test the CPU configuration.
	infos, err := cpu.Info()
	if err != nil {
		s.Fatal("Failed to get CPU info: ", err)
	}
	if n := len(infos); n != 8 {
		s.Errorf("Unexpected number of CPU cores: got %d; want 8", n)
	}
	// The current GCE offering is a stripped Haswell. This is similar to Z840.
	// Matching CPU arch and flags are requested by crosutils/lib/cros_vm_lib.sh.
	// TODO(crbug/998361): Replace with "Haswell, no TSX" once lab is ready.
	const wantArch = "Sandy Bridge"
	for i, info := range infos {
		if !strings.Contains(info.ModelName, wantArch) {
			s.Errorf("Unexpected model name: got %q; want %q", info.ModelName, wantArch)
		}
		// Log the CPU flags for manual investigation.
		// TODO(crbug/1002311): consider long term plan to monitor CPU flags.
		s.Logf("CPU flags #%d: %q", i, info.Flags)
	}

	// Test the RAM configuration.
	m, err := mem.VirtualMemory()
	if err != nil {
		s.Fatal("Failed to get memory info: ", err)
	}
	const minMemory = uint64(7.5 * 1024 * 1024 * 1024) // 7.5GB
	if m.Total < minMemory {
		s.Errorf("Unexpected RAM size: got %d; want >= %d", m.Total, minMemory)
	}
}
