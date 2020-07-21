// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IconAndHostname,
		Desc:     "Test Terminal icon on shelf and hostname of Crostini",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func IconAndHostname(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	// Launch Terminal.
	if err := launcher.SearchAndLaunch(shortCtx, tconn, apps.Terminal.Name); err != nil {
		s.Fatal("Failed to launch Terminal from launcher: ", err)
	}

	// Check Terminal is on shelf.
	if err := ash.WaitForApp(shortCtx, tconn, apps.Terminal.ID); err != nil {
		s.Fatal("Failed to find Terminal icon on shelf: ", err)
	}

	// Check hostname of Crostini.
	cmd := cont.Command(shortCtx, "hostname")
	hostname, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to run command hostname inside container: ", err)
	}
	if strings.TrimRight(string(hostname), "\n") != "penguin" {
		s.Fatal("The hostname of the container is unexpectedly something else: ", string(hostname))
	}
}
