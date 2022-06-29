// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionLog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app session log saves to files successfully",
		Contacts: []string{
			"ashleydp@google.com",
			"zentaro@google.com",
			"menghuan@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			// "gru" is the platform name for scarlet devices. Scarlet
			// needs to be treated differently to handle mobile navigation.
			// TODO(ashleydp): Split tests into "gru" (mobile) and other models.
			hwdep.SkipOnModel("gru")),
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

func getSessionLogPath(ctx context.Context, user string) (string, error) {
	const fileName = "session_log.txt"
	downloadsPath, err := cryptohome.DownloadsPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "Unable to get downloads path for user")
	}
	return path.Join(downloadsPath, fileName), nil
}

func verifySessionLogFile(ctx context.Context, filePath string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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

	// Ensure `DarkLightModeNudge` dismissed before launching app.
	if err := diagnosticsapp.DismissColorModeNudgeIfExists(ctx, tconn); err != nil {
		s.Fatal("Failed to dismiss nudge: ", err)
	}

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

	sessionLogPath, err := getSessionLogPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get session log path for user: ", err)
	}

	if err := verifySessionLogFile(ctx, sessionLogPath); err != nil {
		s.Fatal("Failed to verify that session log file was not empty: ", err)
	}

}
