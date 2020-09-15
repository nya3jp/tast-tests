// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveOk,
		Desc:         "Test uninstalling Crostini via the Settings app",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState"},
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
	})
}

func RemoveOk(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, s.PreValue().(crostini.PreData))

	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Linux Settings: ", err)
	}
	defer st.Close(cleanupCtx)

	// Click Remove on Linus settings page.
	removeDlg, err := st.ClickRemove(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to click Remove button on Linux settings page: ", err)
	}

	// Click Delete on the confirm dialog.
	if err := uig.Do(ctx, tconn, uig.WaitForLocationChangeCompleted(), removeDlg.Delete.LeftClick()); err != nil {
		s.Fatal("Failed to click Delete button on remove Linux dialog: ", err)
	}

	turnOn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, Name: "Linux (Beta)"}, 15*time.Second)
	if err != nil {
		s.Fatal("Failed to find turn on button after removing Linux: ", err)
	}

	turnOn.Release(ctx)
}
