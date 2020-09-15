// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveCancel,
		Desc:         "Test cancelling removal of Crostini from the Settings app",
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

func RemoveCancel(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome

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

	// Click Remove on Linux settings page.
	removeDlg, err := st.ClickRemove(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to click button Remove on Linux settings page: ", err)
	}

	// Click cancel on the confirm dialog.
	if err := uig.Do(ctx, tconn, uig.WaitForLocationChangeCompleted(), removeDlg.Cancel.LeftClick()); err != nil {
		s.Fatal("Failed to click button Cancel on remove Linux dialog: ", err)
	}

	// After cancel remove, settings should stay at Linux.
	if _, err := settings.FindSettingsPage(ctx, tconn, settings.PageNameLinux); err != nil {
		s.Fatal("Failed to find Linux settings page after clicking button Cancel on remove Linux dialog: ", err)
	}

	// Launch Terminal to verify that Crostini still works.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to lauch terminal after cancel remove: ", err)
	}
	terminalApp.Close(ctx)
}
