// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskManager,
		Desc:         "Tests Crostini integration with the task manager",
		Contacts:     []string{"davidmunro@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "download_stretch",
				Pre:       crostini.StartedByDownloadStretch(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "download_buster",
				Pre:       crostini.StartedByDownloadBuster(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func TaskManager(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	defer crostini.RunCrostiniPostTest(ctx, cont)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Couldn't get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Search+\x1b"); err != nil {
		s.Fatal("Couldn't open task manager: ", err)
	}

	s.Log("Find row in task manager")
	taskManagerRootNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:      "Task Manager",
		ClassName: "View",
	}, time.Second*10)
	if err != nil {
		s.Fatal("Couldn't find Task Manager node: ", err)
	}
	defer taskManagerRootNode.Release(ctx)

	entry, err := taskManagerRootNode.DescendantWithTimeout(ctx,
		ui.FindParams{Name: "Linux Virtual Machine: termina"}, time.Second*5)
	if err != nil {
		s.Fatal("Couldn't find node for Crostini: ", err)
	}
	entry.Release(ctx)

	if err := keyboard.Accel(ctx, "Ctrl+w"); err != nil {
		s.Fatal("Couldn't close task manager: ", err)
	}
}
