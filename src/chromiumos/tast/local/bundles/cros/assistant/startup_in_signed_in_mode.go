// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartupInSignedInMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Starts Google Assistant service in signed-in mode and checks the running status",
		Contacts:     []string{"jeroendh@google.com", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		VarDeps:      []string{"assistant.username", "assistant.password"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
	})
}

func StartupInSignedInMode(ctx context.Context, s *testing.State) {
	// Start Chrome browser and log in using a test account.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("assistant.username"),
			Pass: s.RequiredVar("assistant.password"),
		}),
		assistant.VerboseLogging(),
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
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()
}
