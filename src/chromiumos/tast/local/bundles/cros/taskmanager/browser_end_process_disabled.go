// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/taskmanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     BrowserEndProcessDisabled,
		Desc:     "Verify that 'Browser' cannot be killed from Task Manager",
		Contacts: []string{"kevin.wu@cienet.com", "cash.hsu@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "chromeLoggedIn",
	})
}

// BrowserEndProcessDisabled verifies that "Browser" cannot be killed from Task Manager.
func BrowserEndProcessDisabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Open Task Manager")
	tm := taskmanager.New(tconn, kb)
	if err := tm.Open(ctx); err != nil {
		s.Fatal("Failed to open Task Manager: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		tm.Close(ctx, tconn)
	}(cleanupCtx)

	if err := uiauto.Combine("check end process state",
		tm.SelectProcess("Browser"),
		uiauto.New(tconn).CheckRestriction(taskmanager.EndProcessFinder, restriction.Disabled),
	)(ctx); err != nil {
		s.Fatal("Failed to disable end process for Browser: ", err)
	}
}
