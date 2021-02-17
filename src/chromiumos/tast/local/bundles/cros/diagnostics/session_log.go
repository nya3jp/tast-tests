// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SessionLog,
		Desc: "Diagnostics app session log saves to files successfully",
		Contacts: []string{
			"michaelcheco@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func saveButtonDisabled(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return false, err
	}
	params := ui.FindParams{
		Name: "Save",
		Role: ui.RoleTypeButton,
	}
	saveButton, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return false, err
	}

	defer saveButton.Release(ctx)
 	return saveButton.Restriction == ui.RestrictionDisabled, nil

}

func AttemptToSaveSessionLog(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	for i := 0; i < 3; i++ {
		disabled, err := saveButtonDisabled(ctx, tconn)
		if err != nil {
			return false, err
		}

		if (!disabled) {
			if err := ClickSaveButton(ctx, tconn); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func ClickSaveButton(ctx context.Context, tconn *chrome.TestConn) (error) {
	params := ui.FindParams{
		Name: "Save",
		Role: ui.RoleTypeButton,
	}

	saveButton, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return err
	}
	defer saveButton.Release(ctx)

	if err := saveButton.LeftClick(ctx); err != nil {
		return err
	}

	return nil
}

func verifySessionLogFile(ctx context.Context) (error) {
	testing.Sleep(ctx, time.Second)
	filePath := "/home/chronos/user/MyFiles/Downloads/session_log.txt"
	filesInfo, err := os.Stat(filePath)

	if os.IsNotExist(err) {
		return err
	}
	// Remove created session log file
	defer os.Remove(filePath)
	size := filesInfo.Size()

	// Verify that file is not empty.
	if (size == 0) {
		return err
	}
	return nil
}

// SessionLog verifies session log functionality.
func SessionLog(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}
	defer dxRootnode.Release(ctx)

	// Find session log button
	sessionLogButton, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxLogButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the session log button: ", err)
	}
	defer sessionLogButton.Release(ctx)


	// If needed, scroll down to make the session log visible
	if err := sessionLogButton.MakeVisible(ctx); err != nil {
		s.Fatal("Failed to locate session log within the screen bounds: ", err)
	}

	// Click session log button
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := sessionLogButton.StableLeftClick(ctx, &pollOpts); err != nil {
		s.Fatal("Could not click the session log button: ", err)
	}

	if saved, err := AttemptToSaveSessionLog(ctx, tconn); (err != nil || !saved) {
		s.Fatal("Failed to save session log: ", err)
	}

	if err := verifySessionLogFile(ctx); err != nil {
		s.Fatal("Failed to verify that session log file was not empty: ", err)
	}

}
