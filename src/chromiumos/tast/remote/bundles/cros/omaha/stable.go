// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/remote/bundles/cros/omaha/params"
	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Stable,
		Desc: "Check if an appropriate stable version is being served",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{"auto_update_stable"},
		Fixture:      fixture.Omaha,
	})
}

func Stable(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	req := request.New()
	req.GenSP(state.Device, state.Config.OldVersion)
	req.Apps = append(req.Apps, request.GenerateRequestApp(state.Device, state.Config.OldVersion, request.Stable))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := request.Send(ctx, req, "stable")
	if err != nil {
		s.Fatal("Failed to send request: ", err)
	}

	if err := res.ValidateUpdateResponse(); err != nil {
		s.Fatal("Response is not an update: ", err)
	}

	// Check if the current stable is being served. We also accept the next stable
	// here to not break during the transition from Mxx to Mxx+1.
	if chromeVersion, err := res.ChromeVersion(); err != nil {
		s.Error("Failed to get Chrome version: ", err)
	} else if !request.MatchOneOfVersions(chromeVersion, state.Config.CurrentStableChrome, state.Config.NextStableChrome) {
		s.Errorf("Chrome Version %q does not match the current %d or next %d milestone", chromeVersion, state.Config.CurrentStableChrome, state.Config.NextStableChrome)
	}

	currentStableChromeOS := state.Config.ChromeOSVersionFromMilestone[state.Config.CurrentStableChrome]
	nextStableChromeOS := state.Config.ChromeOSVersionFromMilestone[state.Config.NextStableChrome]
	if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
		s.Error("Failed to get ChromeOS version: ", err)
	} else if !request.MatchOneOfVersions(chromeOSVersion, currentStableChromeOS, nextStableChromeOS) {
		s.Errorf("ChromeOS Version %q does not match the current %d or next %d version", chromeOSVersion, currentStableChromeOS, nextStableChromeOS)
	}
}
