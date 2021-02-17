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

func AttemptToSaveSessionLog(ctx context.Context, tconn *chrome.TestConn, s *testing.State) (bool) {
	for i := 0; i < 10; i++ {
		disabled, err := saveButtonDisabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to check disabled state of save button: ", err)
		}

		if (!disabled) {
			if err := ClickSaveButton(ctx, tconn, s); err != nil {
				s.Fatal("Failed to click the save button: ", err)
			}
			return true
		}
	}
	return false
}

func ClickSaveButton(ctx context.Context, tconn *chrome.TestConn, s *testing.State) (error) {
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

	saved := AttemptToSaveSessionLog(ctx, tconn, s)

	if (!saved) {
		s.Fatal("Failed to save session log")
	}

	testing.Sleep(ctx, time.Second)
	// Default file path
	filePath := "/home/chronos/user/MyFiles/Downloads/session_log.txt"
	filesInfo, err := os.Stat(filePath)

	if os.IsNotExist(err) {
		s.Fatal("Session log file does not exist")
	}
	// Remove created session log file
	defer os.Remove(filePath)
	size := filesInfo.Size()

	// Verify that file is not empty.
	if (size == 0) {
		s.Fatal("Session log file is empty")
	}

}
