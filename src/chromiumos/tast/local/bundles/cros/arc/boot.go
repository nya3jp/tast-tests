// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type bootConfig struct {
	// Run boot this many times
	numTrials int
	// Extra args to be paseed to chrome.New
	chromeArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Boot,
		Desc: "Checks that Android boots",
		Contacts: []string{
			"ereth@chromium.org",
			"arc-core@google.com",
			"nya@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: bootConfig{
				numTrials: 1,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_all"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "vm",
			Val: bootConfig{
				numTrials:  1,
				chromeArgs: []string{"--enable-arcvm"},
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "stress",
			Val: bootConfig{
				numTrials: 10,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android"},
			Timeout:           25 * time.Minute,
		}, {
			Name: "vm_stress",
			Val: bootConfig{
				numTrials:  10,
				chromeArgs: []string{"--enable-arcvm"},
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           25 * time.Minute,
		}, {
			Name: "forever",
			Val: bootConfig{
				numTrials: 1000000,
			},
			ExtraAttr:         []string{"disabled"},
			ExtraSoftwareDeps: []string{"android_all"},
			Timeout:           365 * 24 * time.Hour,
		}, {
			Name: "vm_forever",
			Val: bootConfig{
				numTrials:  1000000,
				chromeArgs: []string{"--enable-arcvm"},
			},
			ExtraAttr:         []string{"disabled"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           365 * 24 * time.Hour,
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
	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
		chrome.ExtraArgs(s.Param().(bootConfig).chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Run "pm list packages" and ensure "android" package exists.
	// This ensures package manager service is running at least.
	out, err := a.Command(ctx, "pm", "list", "packages").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("pm list failed: ", err)
	}

	pkgs := strings.Split(string(out), "\n")
	found := false
	for _, p := range pkgs {
		if p == "package:android" {
			found = true
			break
		}
	}

	if !found {
		s.Error("android package not found: ", pkgs)
	}
}
