// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskManager,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests Crostini integration with the task manager",
		Contacts:     []string{"davidmunro@google.com", "clumptini+oncall@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func TaskManager(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(crostini.FixtureData).Tconn
	keyboard := s.FixtValue().(crostini.FixtureData).KB

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	tastkManager := nodewith.Name("Task Manager").ClassName("TaskManagerView").First()
	crostiniEntry := nodewith.Name("Linux Virtual Machine: termina").Ancestor(tastkManager).First()
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("open Task Manager and look for Crostini",
		// Press Search + Esc to launch task manager
		keyboard.AccelAction("Search+Esc"),

		// Click the task manager.
		ui.LeftClick(tastkManager),

		// Focus on Crostini entry.
		ui.WaitUntilExists(crostiniEntry),

		// Exit task manager.
		keyboard.AccelAction("Ctrl+W"))(ctx); err != nil {
		s.Fatal("Failed to test Crostini in Task Manager: ", err)
	}
}
