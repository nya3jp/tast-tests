// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContainerOsRelease,
		Desc:         "UI prompts when crostini OS needs to be upgraded",
		Contacts:     []string{"zubinpratap@google.com", "jinrongwu@google.com", "cros-containers-dev@google.com"},
		VarDeps:      []string{"keepState", "ui.gaiaPoolDefault"},
		Attr:         []string{"group:mainline", "informational"}, //TODO(zubinpratap) promote to mainline if stable
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact(vm.DebianBullseye, false), crostini.GetContainerRootfsArtifact(vm.DebianBullseye, false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcBullseye(),
				Timeout:           14 * time.Minute,
			}, {
				Name:              "unstable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact(vm.DebianBullseye, false), crostini.GetContainerRootfsArtifact(vm.DebianBullseye, false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcBullseye(),
				Timeout:           14 * time.Minute,
			},
		},
	})
}

const (
	releaseFilePath       = "/etc/os-release"
	releaseFileBackupPath = "/etc/backup_os_file"

	// This needs to be updated when a new Debian release is out
	// and we want users to upgrade.
	oldstable = "stretch"
)

var fakeOsReleaseFile = `PRETTY_NAME="Debian GNU/Linux 9 (%[1]s)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (%[1]s)"
VERSION_CODENAME=%[1]s
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

func ContainerOsRelease(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cont := s.PreValue().(crostini.PreData).Container
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	ui := uiauto.New(tconn).WithInterval(500 * time.Millisecond)

	stretchOsReleaseFileContents := fmt.Sprintf(fakeOsReleaseFile, oldstable)
	continueButton := nodewith.Name("Continue anyway").Role(role.Button).First()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, crostini.PostTimeout)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, s.PreValue().(crostini.PreData))

	// Rename the os-release symlink to keep it as backup. Symlink is preserved.
	if err := cont.Command(ctx, "sudo", "mv", releaseFilePath, releaseFileBackupPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rename %q to %q: %v", releaseFilePath, releaseFileBackupPath, err)
	}

	// Create and write THE new /etc/os-release file that mimics debian stretch.
	// This file is not a symlink.
	if err := cont.Command(ctx, "sudo", "sh", "-c", fmt.Sprintf("echo -n %s > %s", shutil.Escape(stretchOsReleaseFileContents), releaseFilePath)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create a new %q file: %v", releaseFilePath, err)
	}

	// Cleanups.
	defer func(ctx context.Context) {
		// Remove the created os-release file
		if err := cont.Command(ctx, "sudo", "rm", releaseFilePath).Run(testexec.DumpLogOnError); err != nil {
			s.Logf("Failed to remove %q on cleanup: %v", releaseFilePath, err)
		}
		// And restore the original symlink.
		if err := cont.Command(ctx, "sudo", "mv", releaseFileBackupPath, releaseFilePath).Run(testexec.DumpLogOnError); err != nil {
			s.Logf("Failed to rename %q to %q: %v", releaseFileBackupPath, releaseFilePath, err)
		}
		s.Logf("cleanup: restored %q with symlink", releaseFilePath)

		// Click on the alert and dismiss it.
		ui.LeftClick(continueButton)(ctx)
		apps.Close(ctx, tconn, apps.Terminal.ID)
	}(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launch the terminal, to fire up Crostini. We need this reference to the terminal so that
	// we can shut down crostini (not just the terminal app) in the next step.
	terminal, err := terminalapp.Launch(ctx, pre.TestAPIConn)
	if err != nil {
		s.Fatalf("Failed to lauch terminal: %q", err)
	}

	// Shutdown crostini container to trigger restart so it reads the new os-release file.
	// That is necessary to show the alert dialog.
	if err := terminal.ShutdownCrostini(cont)(ctx); err != nil {
		s.Fatal("Failed to shutdown crostini: ", err)
	}

	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		s.Fatal("Failed to restart terminal app: ", err)
	}

	// Start Polling for the Alert. Container startup can take time
	// hence the long timeout.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		err := ui.Exists(continueButton)(ctx)
		if err != nil {
			return errors.Wrap(err, "UI alert may not be showing as could not find the \"Continue anyway\" button")
		}
		// Node found.
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})

	if err != nil {
		s.Fatal("upgrade alert ui expected but not found before timeout : ", err)
	}
}
