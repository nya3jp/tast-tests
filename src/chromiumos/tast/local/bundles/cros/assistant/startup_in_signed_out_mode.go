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
		Func:         StartupInSignedOutMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Starts Google Assistant service in signed-out mode and checks the running status",
		Contacts:     []string{"jeroendh@google.com", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		Pre:          assistant.VerboseLoggingEnabled(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

func StartupInSignedOutMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

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
