<<<<<<< HEAD   (725c64 tast-tests: return model name in parsing function)
=======
// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartupInSignedInMode,
		Desc:         "Starts Google Assistant service in signed-in mode and checks the running status",
		Contacts:     []string{"jeroendh@google.com", "xiaohuic@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Vars:         []string{"assistant.username", "assistant.password"},
	})
}

func StartupInSignedInMode(ctx context.Context, s *testing.State) {
	// Start Chrome browser and log in using a test account.
	cr, err := chrome.New(
		ctx,
		chrome.Auth(s.RequiredVar("assistant.username"), s.RequiredVar("assistant.password"), ""),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Create test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer assistant.Disable(ctx, tconn)
}
>>>>>>> CHANGE (e4488d Tast: Stop Assistant after assistant tests)
