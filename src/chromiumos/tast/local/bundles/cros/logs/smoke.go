// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/systemlogs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Smoke,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that writing system logs succeeds",
		Contacts: []string{
			"cros-networking@chromium.org", // Team alias
			"stevenjb@chromium.org",        // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func Smoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const expectedKey = "CHROME VERSION"
	result, err := systemlogs.GetSystemLogs(ctx, tconn, expectedKey)
	if err != nil {
		s.Fatal("Error getting system logs: ", err)
	}
	if result == "" {
		s.Fatal("System logs result empty")
	}
}
