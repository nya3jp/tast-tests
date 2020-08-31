// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenWithTerminal,
		Desc:         "Open directory in FilesApp with terminal",
		Contacts:     []string{"joelhockey@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
				ExtraAttr:         []string{"informational"},
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

func OpenWithTerminal(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx,
		s.PreValue().(crostini.PreData).Container,
		s.PreValue().(crostini.PreData).Chrome.User())

	// Launch Files app and open Downloads with terminal.
	filesApp, err := filesapp.Launch(ctx, pre.TestAPIConn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(ctx)

	if err := filesApp.SelectContextMenu(ctx, "Downloads", "Open with Terminal"); err != nil {
		s.Fatal("Open with Terminal failed: ", err)
	}

	// Find terminal window.
	terminalApp, err := terminalapp.Find(ctx, pre.TestAPIConn, strings.Split(pre.Chrome.User(), "@")[0], "/mnt/chromeos/MyFiles/Downloads")
	if err != nil {
		s.Fatal("Failed to find terminal window: ", err)
	}
	terminalApp.Exit(ctx, pre.Keyboard)
}
