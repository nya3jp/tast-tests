// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package qemu

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sanity,
		Desc: "Set baseline expectations for hosting Chrome OS VM images",
		Contacts: []string{
			"pwang@chromium.org", // Original test author
			"cros-containers-dev@google.com",
			"oka@chromium.org", // Tast port author
		},
		// This test should be kept as informational, because if not informational, as it detects a change in GCE
		// environment, any change to GCE could break the CQ.
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"qemu"},
	})
}

func Sanity(ctx context.Context, s *testing.State) {
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
	for _, info := range infos {
		if !strings.Contains(info.ModelName, wantArch) {
			s.Errorf("Unexpected model name: got %q; want %q", info.ModelName, wantArch)
		}
	}

	// Test the RAM configuration.
	m, err := mem.VirtualMemory()
	if err != nil {
		s.Fatal("Failed to get memory info: ", err)
	}
	const minMemoryKB = uint64(7.5 * 1024 * 1024) // 7.5GB
	if got := m.Total / 1024; got < minMemoryKB {
		s.Errorf("Unexpected RAM size: got %dKb; want >= %dKb", got, minMemoryKB)
	}
}
