// Copyright 2021 The ChromiumOS Authors
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

	prevVersion, err := state.Config.PreviousMilestoneOSVersion(state.Config.CurrentChromeOSStable())
	if err != nil {
		s.Fatal("Failed to get previous version: ", err)
	}

	req := request.New()
	req.GenSP(state.Device, prevVersion)
	req.Apps = append(req.Apps, request.GenerateRequestApp(state.Device, prevVersion, request.Stable))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := request.Send(ctx, req, "stable")
	if err != nil {
		s.Fatal("Failed to send request: ", err)
	}

	if err := res.ValidateUpdateResponse(); err != nil {
		s.Fatal("Response is not an update: ", err)
	}

	prevMilestone, err := state.Config.PreviousMilestone(state.Config.CurrentChromeOSStable())
	if err != nil {
		s.Fatal("Failed to get previous version: ", err)
	}

	// Check if the current stable is being served. We also accept the previous
	// stable here to not break during fractional pushes.
	if chromeVersion, err := res.ChromeVersion(); err != nil {
		s.Error("Failed to get Chrome version: ", err)
	} else if !request.MatchOneOfVersions(chromeVersion, state.Config.CurrentChromeOSStable(), prevMilestone) {
		s.Errorf("Chrome Version %q does not match the current %d or previous %d milestone", chromeVersion, state.Config.CurrentChromeOSStable(), prevMilestone)
	}

	currentStableChromeOS := state.Config.ChromeOSVersionFromMilestone[state.Config.CurrentChromeOSStable()]
	prevStableChromeOS := state.Config.ChromeOSVersionFromMilestone[prevMilestone]
	if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
		s.Error("Failed to get ChromeOS version: ", err)
	} else if !request.MatchOneOfVersions(chromeOSVersion, currentStableChromeOS, prevStableChromeOS) {
		s.Errorf("ChromeOS Version %q does not match the current %d or previous %d version", chromeOSVersion, currentStableChromeOS, prevStableChromeOS)
	}
}
