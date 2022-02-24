// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/remote/bundles/cros/omaha/params"
	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MajorVersionPinning,
		Desc: "Check if an appropriate versions are served when pinning to a major version",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:    []string{"group:omaha"},
		Fixture: fixture.Omaha,
	})
}

func MajorVersionPinning(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	req := request.New()
	req.GenSP(state.Device, state.Config.OldVersion)
	requestApp := request.GenerateRequestApp(state.Device, state.Config.OldVersion, request.Stable)

	for vpMilestone, vpChromeOSVersion := range state.Config.ChromeOSVersionFromMilestone {
		if vpMilestone > state.Config.CurrentStableChrome {
			s.Logf("Not testing version %d higher than stable %d", vpMilestone, state.Config.CurrentStableChrome)
			continue
		}

		// Major Version Pinning only supports the last 10 milestones.
		if vpMilestone < state.Config.CurrentStableChrome-10 {
			s.Logf("Not testing version %d, too old", vpMilestone)
			continue
		}

		vpMilestoneStr := strconv.FormatInt(int64(vpMilestone), 10)
		vpChromeOSVersionStr := strconv.FormatInt(int64(vpChromeOSVersion), 10)

		s.Run(ctx, "M"+vpMilestoneStr, func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			requestApp.UpdateCheck.TargetVersionPrefix = vpChromeOSVersionStr
			req.Apps = []request.RequestApp{requestApp}

			res, err := request.Send(ctx, req, "M"+vpMilestoneStr)
			if err != nil {
				s.Fatal("Failed to send request: ", err)
			}

			if err := res.ValidateUpdateResponse(); err != nil {
				s.Fatal("Response is not an update: ", err)
			}

			// Check if the requested milestone is being served.
			if chromeVersion, err := res.ChromeVersion(); err != nil {
				s.Error("Failed to get Chrome version: ", err)
			} else if !request.MatchOneOfVersions(chromeVersion, vpMilestone) {
				s.Errorf("Chrome Version %q does not match the expected milestone %d", chromeVersion, vpMilestone)
			}

			if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
				s.Error("Failed to get ChromeOS version: ", err)
			} else if !request.MatchOneOfVersions(chromeOSVersion, vpChromeOSVersion) {
				s.Errorf("ChromeOS Version %q does not match the expected prefix %d", chromeOSVersion, vpChromeOSVersion)
			}
		})
	}
}
