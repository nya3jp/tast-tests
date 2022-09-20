// Copyright 2022 The ChromiumOS Authors
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
		Func:         EnableAndDisableMultipleTimes,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enable/disable Google Assistant service multiple times and checks the running status",
		Contacts:     []string{"wutao@google.com", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          assistant.VerboseLoggingEnabled(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      3 * time.Minute,
	})
}

func EnableAndDisableMultipleTimes(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Create test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable and disable the Assistant multiple times. Should not crash.
	// The max number 30 is an arbitrary number.
	// With the bug, it will crash with about 10 - 20 times of toggles.
	// Please see b/233157402.
	const maxNumOfToggles = 30
	for toggle := 0; toggle <= maxNumOfToggles; toggle++ {
		testing.ContextLog(ctx, "Enabling Assistant, toggle ", toggle)
		if err := assistant.Enable(ctx, tconn); err != nil {
			s.Fatal("Failed to enable Assistant: ", err)
		}
		testing.ContextLog(ctx, "Disabling Assistant, toggle ", toggle)
		if err := assistant.Disable(ctx, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to clean up Assistant: ", err)
		}
	}()
}
