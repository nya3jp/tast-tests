// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DebianUpgradeAlert,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "UI prompts when crostini OS needs to be upgraded",
		Contacts:     []string{"clumptini+oncall@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			// Parameters generated by debian_upgrade_alert_test.go. DO NOT EDIT.
			{
				Name:              "stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           14 * time.Minute,
			}, {
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
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

func DebianUpgradeAlert(ctx context.Context, s *testing.State) {
	cont := s.FixtValue().(crostini.FixtureData).Cont
	tconn := s.FixtValue().(crostini.FixtureData).Tconn
	cr := s.FixtValue().(crostini.FixtureData).Chrome

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	ui := uiauto.New(tconn).WithInterval(500 * time.Millisecond)
	continueButton := nodewith.Name("Continue anyway").Role(role.Button).First()

	// Cleanups.
	defer func(ctx context.Context) {
		// Remove the created os-release file
		if err := cont.Command(ctx, "sudo", "rm", "-f", releaseFilePath).Run(testexec.DumpLogOnError); err != nil {
			s.Logf("Cleanup: failed to remove %q on cleanup: %v", releaseFilePath, err)
		}
		// And restore the original symlink.
		if err := cont.Command(ctx, "sudo", "mv", releaseFileBackupPath, releaseFilePath).Run(testexec.DumpLogOnError); err != nil {
			s.Logf("Cleanup: failed to rename %q to %q: %v", releaseFileBackupPath, releaseFilePath, err)
		}
		s.Logf("Cleanup: restored %q with symlink", releaseFilePath)

		// The test passes if the alert shows when terminal is being fired up.
		// At this point, there is an open terminal.  Find it and
		// shutdown crostini so that os file changes are picked up.
		tApp, err := terminalapp.Find(ctx, tconn)
		if err != nil {
			s.Log("Cleanup: failed to find terminal app: ", err)
		}

		if err := tApp.ShutdownCrostini(cont)(ctx); err != nil {
			s.Log("Cleanup: failed to shutdown crostini: ", err)
		}
		s.Log("Cleanup: shut down crostini completed")
	}(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Rename the os-release symlink to keep it as backup. Symlink is preserved.
	if err := cont.Command(ctx, "sudo", "mv", releaseFilePath, releaseFileBackupPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rename %q to %q: %v", releaseFilePath, releaseFileBackupPath, err)
	}

	// Create and write the new /etc/os-release file that mimics debian stretch.
	// This file is not a symlink.
	stretchOsReleaseFileContents := fmt.Sprintf(fakeOsReleaseFile, oldstable)
	if err := cont.Command(ctx, "sudo", "sh", "-c", fmt.Sprintf("echo -n %s > %s", shutil.Escape(stretchOsReleaseFileContents), releaseFilePath)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create a new %q file: %v", releaseFilePath, err)
	}

	// Launch the terminal. We need this reference to the terminal so that
	// we can shut down crostini (not just the terminal app) in the next step.
	terminal, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch terminal: ", err)
	}

	// Shutdown crostini container to trigger restart so it reads the new os-release file.
	// That is necessary to show the alert dialog.
	if err := terminal.ShutdownCrostini(cont)(ctx); err != nil {
		s.Fatal("Failed to shutdown crostini: ", err)
	}

	// Launch terminal and click 'penguin' link on home tab.
	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		s.Fatal("Failed to restart terminal app: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("penguin").Role(role.Link))(ctx); err != nil {
		s.Fatal("Failed to click Terminal Home Linux: ", err)
	}

	if err := testing.Poll(
		ctx,
		uiauto.Combine("Find the upgrade alert",
			// Reset automation to refresh the UI tree as UI tree can lag
			// behind the display causing the test to fail incorrectly.
			tconn.ResetAutomation,
			ui.WithTimeout(time.Second).WaitUntilExists(continueButton),
		),
		&testing.PollOptions{Timeout: terminalapp.LaunchTerminalTimeout},
	); err != nil {
		s.Fatal("Failed to find the upgrade alert before timeout: ", err)
	}

	s.Log("Found the Debian upgrade popup alert")

	// Click on the alert and dismiss it before proceeding.
	if err := ui.LeftClick(continueButton)(ctx); err != nil {
		s.Log("Failed to find or click the continue button on the alert: ", err)
	}

	err = ui.EnsureGoneFor(continueButton, 5*time.Second)(ctx)
	if err != nil {
		// error is wanted because the alert should no longer be present.
		s.Fatal("Upgrade alert pop should have been dismissed and not be present on screen")
	}

	// Check Terminal app is present.
	_, err = terminalapp.Find(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find open terminal app after dismissing the alert button: ", err)
	}

	st, err := settings.OpenLinuxSettings(ctx, tconn, cr)
	defer st.Close(ctx)
	if err != nil {
		s.Fatal("Failed to open Linux Settings: ", err)
	}

	if err := st.WaitForUI(settings.DebianUpgradeText)(ctx); err != nil {
		s.Fatal("Failed to see upgrade alert text in Linux settings: ", err)
	}
}
