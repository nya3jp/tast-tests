// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantStartupInSignedOutMode,
		Desc:         "Starts Google Assistant service in signed-out mode and checks the running status",
		Contacts:     []string{"jeroendh@google.com", "xiaohuic@chromium.org"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline"},
	})
}

func AssistantStartupInSignedOutMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.WaitForServiceReadySignal(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
}
