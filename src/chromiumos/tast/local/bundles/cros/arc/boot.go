// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

type bootConfig struct {
	// Run boot this many times
	numTrials int
	// Use O_DIRECT in disk access for ARCVM
	oDirect bool
	// Extra Chrome command line options
	chromeArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Boot,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that Android boots",
		Contacts: []string{
			"arc-core@google.com",
			"nya@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: bootConfig{
				numTrials: 1,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_p"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "forever",
			Val: bootConfig{
				numTrials: 1000000,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Timeout:           365 * 24 * time.Hour,
		}, {
			Name: "stress",
			Val: bootConfig{
				numTrials: 10,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			Timeout:           25 * time.Minute,
		}, {
			Name: "vm",
			Val: bootConfig{
				numTrials: 1,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "vm_with_per_vcpu_core_scheduling",
			Val: bootConfig{
				numTrials: 1,
				// Switch from per-VM core scheduling to per-vCPU core scheduling which
				// is more secure but slow.
				chromeArgs: []string{"--disable-features=ArcEnablePerVmCoreScheduling"},
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "vm_forever",
			Val: bootConfig{
				numTrials: 1000000,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           365 * 24 * time.Hour,
		}, {
			Name: "vm_o_direct",
			Val: bootConfig{
				numTrials: 1,
				oDirect:   true,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "vm_stress",
			Val: bootConfig{
				numTrials: 10,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           25 * time.Minute,
		}, {
			Name: "vm_large_memory",
			Val: bootConfig{
				numTrials: 1,
				// Boot ARCVM with the largest possible guest memory size.
				chromeArgs: []string{"--enable-features=ArcVmMemorySize:shift_mib/0"},
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
		}},
	})
}

func Boot(ctx context.Context, s *testing.State) {
	numTrials := s.Param().(bootConfig).numTrials
	for i := 0; i < numTrials; i++ {
		if numTrials > 1 {
			s.Logf("Trial %d/%d", i+1, numTrials)
		}
		runBoot(ctx, s)
	}
}

func runBoot(ctx context.Context, s *testing.State) {
	if s.Param().(bootConfig).oDirect {
		if err := arc.WriteArcvmDevConf(ctx, "O_DIRECT=true"); err != nil {
			s.Fatal("Failed to set arcvm_dev.conf: ", err)
		}
		defer arc.RestoreArcvmDevConf(ctx)
	}

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	args := s.Param().(bootConfig).chromeArgs
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while booting ARC: ", err)
		}
	}()

	a, err := arc.NewWithSyslogReader(ctx, s.OutDir(), reader)
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	// Ensures package manager service is running by checking the existence of the "android" package.
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Getting installed packages failed: ", err)
	}

	if _, ok := pkgs["android"]; !ok {
		s.Fatal("android package not found: ", pkgs)
	}
}
