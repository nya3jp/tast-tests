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
	"chromiumos/tast/local/input"
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

	params := ui.FindParams{
		Name: "Save",
		Role: ui.RoleTypeButton,
	}
	saveButton, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the save button: ", err)
	}
	defer saveButton.Release(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := saveButton.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to focus the save button: ", err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type enter: ", err)
	}
	
	filePath := "/home/chronos/user/MyFiles/Downloads/session_log.txt"
	filesInfo, err := os.Stat(filePath)
	
	if os.IsNotExist(err) {
		s.Fatal("File was not found")
	}

	var size int64
	if (filesInfo != nil) {
		size = filesInfo.Size()
	}

	if (size == 0) {
		s.Fatal("Session log file is empty")
	}

	os.Remove(filePath)
}
