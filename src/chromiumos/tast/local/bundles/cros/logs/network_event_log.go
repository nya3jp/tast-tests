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
		Func: NetworkEventLog,
		Desc: "Tests that writing system logs for feedback reports succeeds",
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
	var logs string
	if logs, err = systemlogs.GetMultilineSection(ctx, tconn, networkSection); err != nil {
		s.Fatal(errors.Wrap(err, "failed to read network_event_log fro system logs"))
	}

	const errorKey = "ERROR"
	lines := strings.Split(logs, "\n")
	for _, l := range lines {
		if idx := strings.Index(l, errorKey); idx != -1 {
			s.Errorf("Unexpected ERROR line: %s", l)
		}
	}
}
