// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosvm

import (
	"context"
	"reflect"
	"sort"

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
		Attr: []string{"informational"},
		// TODO(oka): Add software deps to run this test only on VMs.
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	// Test the CPU configuration.
	is, err := cpu.Info()
	if err != nil {
		s.Fatal("Failed to get CPU info: ", err)
	}
	if n := len(is); n != 8 {
		s.Errorf("Found %d CPU cores, where 8 was expected", n)
	}
	// The current GCE offering is a stripped Haswell. This is similar to Z840.
	// Matching CPU arch and flags are requested by crosutils/lib/cros_vm_lib.sh.
	// TODO(pwang): Replace with "Haswell, no TSX" once lab is ready.
	// FIXME(oka): pwang@, could you file a bug for it?
	const wantArch = "Sandy Bridge"
	// These are flags that sampled from GCE builders on cros lab.
	wantFlags := []string{
		"abm", "aes", "apic", "arat", "avx", "avx2", "bmi1", "bmi2",
		"clflush", "cmov", "constant_tsc", "cx16", "cx8", "de", "eagerfpu",
		"erms", "f16c", "fma", "fpu", "fsgsbase", "fxsr", "hypervisor",
		"kaiser", "lahf_lm", "lm", "mca", "mce", "mmx", "movbe", "msr",
		"mtrr", "nopl", "nx", "pae", "pat", "pcid", "pclmulqdq", "pge",
		"pni", "popcnt", "pse", "pse36", "rdrand", "rdtscp", "rep_good",
		"sep", "smep", "sse", "sse2", "sse4_1", "sse4_2", "ssse3",
		"syscall", "tsc", "vme", "x2apic", "xsave", "xsaveopt",
	}
	for _, i := range is {
		if i.ModelName != wantArch {
			s.Errorf("Found modle %q, where %q was expected", i.ModelName, wantArch)
		}
		sort.Strings(i.Flags)
		if !reflect.DeepEqual(i.Flags, wantFlags) {
			// TODO(oka): disable this check if it fails, following this comment in the original test:
			// # TODO(pwang): convert warning to error once VM got better infra support.
			s.Errorf("Found CPU flags %q, where %q were expected", i.Flags, wantFlags)
		}
	}

	// TODO(pwang): Add check once virgl is fully introduced to VM.
	// FIXME(oka): Check if this TODO to implement GPU test should be left in the code. pwang@ WDYT?

	// Test the RAM configuration.
	m, err := mem.VirtualMemory()
	if err != nil {
		s.Fatal("Failed to get memory info: ", err)
	}
	const minMemoryKB = uint64(7.5 * 1024 * 1024) // 7.5GB
	if got := m.Total / 1024; got < minMemoryKB {
		s.Errorf("Found %dKB of memory where at least %dKB was expected", got, minMemoryKB)
	}
}
