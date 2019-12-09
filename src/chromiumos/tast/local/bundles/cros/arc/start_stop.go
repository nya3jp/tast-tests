// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/startstop"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StartStop,
		Desc: "Verifies clean start and stop of CrOS Chrome and Android container",
		Contacts: []string{
			// Contacts for TestPID and TestMount failure.
			"rohitbm@chromium.org", // Original author.
			"arc-eng@google.com",

			// Contacts for TestMidis.
			"pmalani@chromium.org", // original author
			"chromeos-audio@google.com",

			"hidehiko@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Val: []startstop.Subtest{
				&startstop.TestPID{},
				&startstop.TestMount{},
				&startstop.TestSysctl{},
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: []startstop.Subtest{
				&startstop.TestPID{},
			},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_both"},
			Val: []startstop.Subtest{
				&startstop.TestMidis{},
			},
		}},
	})
}

func StartStop(ctx context.Context, s *testing.State) {
	tests := s.Param().([]startstop.Subtest)

	// Restart ui job to ensure starting from logout state.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}
	for _, t := range tests {
		s.Run(ctx, t.Name()+".PreStart", t.PreStart)
	}

	// Launch Chrome with enabling ARC.
	func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()

		for _, t := range tests {
			s.Run(ctx, t.Name()+".PostStart", t.PostStart)
		}
	}()

	// Log out from Chrome, which shuts down ARC.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}
	for _, t := range tests {
		s.Run(ctx, t.Name()+".PostStop", t.PostStop)
	}
}
