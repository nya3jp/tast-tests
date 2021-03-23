// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Backup,
		Desc:         "This test ensure that we can backup and restore Android Apps",
		Contacts:     []string{"rohitbm@google.com", "arc-core@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func Backup(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcBackupTest.apk"
		pkg = "org.chromium.arc.testapp.backup"
		cls = pkg + ".BackupActivity"

		editMessageID = pkg + ":id/edit_message"
		saveID        = pkg + ":id/save_button"
		loadID        = pkg + ":id/load_button"
		clearID       = pkg + ":id/clear_button"
		fileContentID = pkg + ":id/file_content"
		successText   = "Success"
		failureText   = "Fail"

		defaultTimeout = 30 * time.Second
	)

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	filename := fmt.Sprintf("%v", time.Now().UnixNano())
	s.Log("Filename: ", filename)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Save and load.
	editMessage := d.Object(ui.ID(editMessageID))
	if err := editMessage.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Fatal("Failed to wait for edit message to exist: ", err)
	}
	if err := editMessage.SetText(ctx, filename); err != nil {
		s.Fatal("Failed to set edit message text: ", err)
	}

	save := d.Object(ui.ID(saveID))
	if err := save.Click(ctx); err != nil {
		s.Fatal("Failed to click save: ", err)
	}
	load := d.Object(ui.ID(loadID))
	if err := load.Click(ctx); err != nil {
		s.Fatal("Failed to click load: ", err)
	}

	// Ensure success.
	successContent := d.Object(ui.ID(fileContentID), ui.TextContains("Success"))
	if err := successContent.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Fatal("Failed to wait for success file content to exist: ", err)
	}

	// Get previous backup count.
	startingBackupCount, err := everBackedUp(ctx, a)
	if err != nil {
		s.Fatal("Failed to check if ever backed up: ", err)
	}

	// Run a backup.
	if err := a.Command(ctx, "bmgr", "backupnow", pkg).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run backup command: ", err)
	}

	s.Log("Polling until backup completes")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		backupCount, err := everBackedUp(ctx, a)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check backup count"))
		}
		if backupCount != startingBackupCount+1 {
			return errors.Errorf("waiting for backup to complete: (got: %d, want: %d)", backupCount, startingBackupCount+1)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Backup failed to complete: ", err)
	}

	// Get restore token.
	output, err := a.Command(ctx, "dumpsys", "backup").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to dump backup info for restore token: ", err)
	}
	re := regexp.MustCompile(`(?:Current:   )([0-9a-f]*)`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		s.Fatal("Failed to match restore token regex")
	}
	restoreToken := groups[1]
	s.Log("Got restore token: ", restoreToken)

	if backupCount, err := everBackedUp(ctx, a); err != nil {
		s.Fatal("Failed to check back up count: ", err)
	} else if backupCount != startingBackupCount+1 {
		s.Fatalf("Another backup happened concurrently: (got: %d, want: %d)", backupCount, startingBackupCount+1)
	}

	// Relaunch the application because it closes during backup.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	// Clear and load.
	if err := editMessage.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Fatal("Failed to wait for edit message to exist: ", err)
	}
	if err := editMessage.SetText(ctx, filename); err != nil {
		s.Fatal("Failed to set edit message text: ", err)
	}

	clear := d.Object(ui.ID(clearID))
	if err := clear.Click(ctx); err != nil {
		s.Fatal("Failed to click save: ", err)
	}
	if err := load.Click(ctx); err != nil {
		s.Fatal("Failed to click load: ", err)
	}

	// Ensure failure.
	failContent := d.Object(ui.ID(fileContentID), ui.TextContains(failureText))
	if err := failContent.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for failure file content to exist: ", err)
	}

	// Restore from backup.
	if err := a.Command(ctx, "bmgr", "restore", restoreToken, pkg).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run backup command: ", err)
	}

	s.Log("Polling until restore completes")
	if err := a.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile("Restore complete.*org.chromium.arc.testapp.backup"))); err != nil {
		s.Fatal("Restore failed to complete: ", err)
	}

	// Relaunch the application because it closes during restore.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	// Load and ensure success.
	if err := editMessage.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Fatal("Failed to wait for edit message to exist: ", err)
	}
	if err := editMessage.SetText(ctx, filename); err != nil {
		s.Fatal("Failed to set edit message text: ", err)
	}

	if err := load.Click(ctx); err != nil {
		s.Fatal("Failed to click load: ", err)
	}
	if err := successContent.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for success file content to exist: ", err)
	}
}

func everBackedUp(ctx context.Context, a *arc.ARC) (int64, error) {
	output, err := a.Command(ctx, "dumpsys", "backup").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to dump backup info")
	}
	re := regexp.MustCompile(`(?:Ever backed up: )([0-9]*)`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		return 0, errors.New("failed to match ever backed up regex")
	}
	everBackedUp, err := strconv.ParseInt(groups[1], 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert ever backed up to int: ")
	}
	return everBackedUp, nil
}
