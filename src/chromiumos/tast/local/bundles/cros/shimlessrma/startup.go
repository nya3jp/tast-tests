// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains drivers for controlling the ui of Shimless
// RMA SWA.
package shimlessrma

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Startup,
		Desc: "Can successfully open the Shimless RMA app",
		Contacts: []string{
			"gavindodd@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// Startup verifies that the Shimless RMA app can open.
func Startup(ctx context.Context, s *testing.State) {
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

	if err := apps.Launch(ctx, tconn, apps.ShimlessRma.ID); err != nil {
		s.Fatal("Failed to launch Shimless RMA app: ", err)
	}
}
