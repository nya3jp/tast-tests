// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC06T1GoogleMeetConference,
		Desc:         "Using Google Meet host a conference and presentation with participants",
		Contacts:     []string{"jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.GoogleMeetService",
			"tast.cros.cuj.LocalStoreService",
		},
		Vars:    []string{"ui.cuj_username", "ui.cuj_password", "ui.meet_url", "perf_level"},
		Timeout: time.Minute * 12,
	})
}

func TC06T1GoogleMeetConference(ctx context.Context, s *testing.State) {
	perfLevel := s.RequiredVar("perf_level")
	conference.Run(ctx, s, conference.GoogleMeet, perfLevel, conference.OneRoomSize)
}
