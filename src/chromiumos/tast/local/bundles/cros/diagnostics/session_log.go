// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
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

func attemptToSaveSessionLog(ctx context.Context, tconn *chrome.TestConn) error {
	if err := saveButtonDisabled(ctx, tconn); err != nil {
		return err
	}

	if err := clickSaveButton(ctx, tconn); err != nil {
		return err
	}
	return nil
}

func clickSaveButton(ctx context.Context, tconn *chrome.TestConn) error {
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

func saveButtonDisabled(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Name: "Save",
		Role: ui.RoleTypeButton,
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		saveButton, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "Unable to get save button"))
		}
		defer saveButton.Release(ctx)
		if saveButton.Restriction == ui.RestrictionDisabled {
			return errors.Errorf("Save button state %s", saveButton.Restriction)
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "Save button failed to change state")
	}
	return nil
}

func verifySessionLogFile(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const filePath = "/home/chronos/user/MyFiles/Downloads/session_log.txt"
		filesInfo, err := os.Stat(filePath)

		if os.IsNotExist(err) {
			return err
		}
		// Remove created session log file.
		defer os.Remove(filePath)
		size := filesInfo.Size()

		// Verify that file is not empty.
		if size == 0 {
			return testing.PollBreak(err)
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 2 * time.Second}); err != nil {
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

	// Find session log button.
	sessionLogButton, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxLogButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the session log button: ", err)
	}
	defer sessionLogButton.Release(ctx)

	// If needed, scroll down to make the session log visible.
	if err := sessionLogButton.MakeVisible(ctx); err != nil {
		s.Fatal("Failed to locate session log within the screen bounds: ", err)
	}

	// Click session log button.
	if err := sessionLogButton.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}); err != nil {
		s.Fatal("Could not click the session log button: ", err)
	}

	if err := attemptToSaveSessionLog(ctx, tconn); err != nil {
		s.Fatal("Failed to save session log: ", err)
	}

	if err := verifySessionLogFile(ctx); err != nil {
		s.Fatal("Failed to verify that session log file was not empty: ", err)
	}

}
