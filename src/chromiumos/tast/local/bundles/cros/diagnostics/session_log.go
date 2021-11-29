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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionLog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app session log saves to files successfully",
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
	ui := uiauto.New(tconn)
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("click Save",
		ui.WithTimeout(10*time.Second).WaitUntilExists(saveButton),
		ui.LeftClick(saveButton),
	)(ctx); err != nil {
		return err
	}

	return nil
}

func saveButtonDisabled(ctx context.Context, tconn *chrome.TestConn) error {
	saveButton := nodewith.Name("Save").Role(role.Button)
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(saveButton)(ctx); err != nil {
		return errors.Wrap(err, "Unable to get save button")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.CheckRestriction(saveButton, restriction.Disabled)(ctx); err == nil {
			return errors.Errorf("Save button state %s", restriction.Disabled)
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

	// Find session log button. If needed, scroll down to make the session log visible and Click session log button.
	ui := uiauto.New(tconn)
	sessionLogButton := diagnosticsapp.DxLogButton.Ancestor(dxRootnode)
	if err := uiauto.Combine("find and click session log button",
		ui.WithTimeout(20*time.Second).WaitUntilExists(sessionLogButton),
		ui.MakeVisible(sessionLogButton),
		ui.WithPollOpts(testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}).LeftClick(sessionLogButton),
	)(ctx); err != nil {
		s.Fatal("Could not click the session log button: ", err)
	}

	if err := attemptToSaveSessionLog(ctx, tconn); err != nil {
		s.Fatal("Failed to save session log: ", err)
	}

	if err := verifySessionLogFile(ctx); err != nil {
		s.Fatal("Failed to verify that session log file was not empty: ", err)
	}

}
