// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains drivers for controlling the ui of Shimless
// RMA SWA.
package shimlessrma

import (
	"context"
	"os"
	"os/user"
	"strconv"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WelcomeCancel,
		Desc: "Can successfully start and cancel the Shimless RMA app",
		Contacts: []string{
			"gavindodd@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

const (
	stateFile = "/mnt/stateful_partition/unencrypted/rma-data/state"
)

// WelcomeCancel verifies that the Shimless RMA app can open and immediately
// cancel.
func WelcomeCancel(ctx context.Context, s *testing.State) {
	// Get the rmad user id and group.
	u, err := user.Lookup("rmad")
	if err != nil {
		s.Fatal("Failed to get rmad user: ", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		s.Fatal("Failed to get rmad uid: ", err)
	}

	// Create a valid empty rmad state file.
	f, err := os.Create(stateFile)
	if err != nil {
		s.Fatal("Failed to create rmad state file: ", err)
	}
	l, err := f.WriteString("{}\n")
	if err != nil || l < 3 {
		s.Fatal("Failed to write to rmad state file: ", err)
	}
	if err := f.Chown(uid, -1); err != nil {
		s.Fatal("Failed to set rmad as owner of state file: ", err)
	}
	if err := f.Close(); err != nil {
		s.Fatal("Failed to write to rmad state file: ", err)
	}
	defer os.Remove(stateFile)

	// Open Chrome with Shimless RMA enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ShimlessRMAFlow"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open Shimless RMA app.
	if err := apps.Launch(ctx, tconn, apps.ShimlessRma.ID); err != nil {
		s.Fatal("Failed to launch Shimless RMA app: ", err)
	}

	ui := uiauto.New(tconn)
	rootNode := nodewith.Name(apps.ShimlessRma.Name).Role(role.Window)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(rootNode)(ctx); err != nil {
		s.Fatal("Failed to find Shimless RMA app root node: ", err)
	}

	// Click the cancel button
	cancelButton := nodewith.Name("Cancel").Role(role.Button).Ancestor(rootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(cancelButton)(ctx); err != nil {
		s.Fatal("Failed to find cancel button: ", err)
	}
	if err := ui.LeftClick(cancelButton)(ctx); err != nil {
		s.Fatal("Failed to click cancel button: ", err)
	}

	// Confirm RMA cancel completed and deleted the state file.
	const pauseDuration = time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("Failed to sleep while waiting for RMA to cancel: ", err)
	}

	if _, err := os.Stat(stateFile); err == nil {
		s.Fatal("Failed to cancel RMA, state file not deleted")
	} else if !os.IsNotExist(err) {
		s.Fatal("Failed to confirm RMA state file does not exist: ", err)
	}
}
