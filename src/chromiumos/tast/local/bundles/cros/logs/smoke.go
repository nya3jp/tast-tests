// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

import (
	"context"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/systemlogs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Smoke,
		Desc: "Tests that writing system logs succeeds",
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

	var logs string
	if logs, err = systemlogs.GetSystemLogs(ctx, tconn); err != nil {
		s.Fatal("System logs not written: ", err)
	}
	if logs == "" {
		s.Fatal("System logs empty")
	}
	expectedKey := "CHROME VERSION"
	if !strings.Contains(logs, expectedKey) {
		s.Fatal("System logs missing: ", expectedKey)
	}
}
