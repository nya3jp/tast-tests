// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/systemlogs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkEventLog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the network_event_log section of the system logs has no ERROR entries",
		Contacts: []string{
			"cros-networking@chromium.org", // Team alias
			"stevenjb@chromium.org",        // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func NetworkEventLog(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const networkSection = "network_event_log"
	logs, err := systemlogs.GetSystemLogs(ctx, tconn, networkSection)
	if err != nil {
		s.Fatal(errors.Wrap(err, "failed to read network_event_log from system logs"))
	}

	const errorKey = "ERROR"
	lines := strings.Split(logs, "\n")
	if len(lines) < 2 {
		s.Fatalf("Too few lines in result: %s", logs)
	}
	for _, l := range lines {
		if strings.Contains(l, errorKey) {
			s.Errorf("Unexpected ERROR line: %s", l)
		}
	}
}
