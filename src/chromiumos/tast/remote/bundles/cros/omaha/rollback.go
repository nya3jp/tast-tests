// Copyright 2021 The ChromiumOS Authors
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
		Func: Rollback,
		Desc: "Check if an appropriate versions are served when rolling back to a major version",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{"auto_update_stable"},
		Fixture:      fixture.Omaha,
	})
}

func Rollback(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	chromeOSStableInt := state.Config.ChromeOSVersionFromMilestone[state.Config.CurrentChromeOSStable()]
	chromeOSStable := strconv.FormatInt(int64(chromeOSStableInt), 10) + ".0.0"

	req := request.New()
	req.GenSP(state.Device, chromeOSStable)
	requestApp := request.GenerateRequestApp(state.Device, chromeOSStable, request.Stable)

	// Go trough all possible major version pins.
	for vpMilestone, vpChromeOSVersion := range state.Config.ChromeOSVersionFromMilestone {
		if vpMilestone > state.Config.CurrentChromeOSStable() {
			s.Logf("Not testing version %d higher than stable %d", vpMilestone, state.Config.CurrentChromeOSStable())
			continue
		}
		vpMilestoneStr := strconv.FormatInt(int64(vpMilestone), 10)
		vpChromeOSVersionStr := strconv.FormatInt(int64(vpChromeOSVersion), 10)

		if vpMilestone != state.Config.CurrentChromeOSStable() {
			// Check that rollback is not sent when not requested.
			s.Run(ctx, "M"+vpMilestoneStr+"_no_rollback", func(ctx context.Context, s *testing.State) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				requestApp.UpdateCheck.TargetVersionPrefix = vpChromeOSVersionStr
				requestApp.UpdateCheck.RollbackAllowed = false
				req.Apps = []request.RequestApp{requestApp}

				res, err := request.Send(ctx, req, "M"+vpMilestoneStr+"_no_rollback")
				if err != nil {
					s.Fatal("Failed to send request: ", err)
				}

				if err := res.ValidateUpdateResponse(); err == nil {
					if res.App.UpdateCheck.Rollback {
						s.Error("Response is an actual rollback")
					}

					if chromeVersion, err := res.ChromeVersion(); err != nil {
						s.Error("Got an update to an older version without requesting rollback, failed to get version: ", err)
					} else {
						s.Error("Got an update to an older version without requesting rollback: ", chromeVersion)
					}

				}
			})
		}

		// Rollback only supports the last 4 milestones.
		if vpMilestone < state.Config.CurrentChromeOSStable()-4 {
			s.Logf("Not testing version %d, too old", vpMilestone)
			continue
		}

		// Check that we are getting the expected version when requesting rollback.
		s.Run(ctx, "M"+vpMilestoneStr+"_rollback", func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			requestApp.UpdateCheck.TargetVersionPrefix = vpChromeOSVersionStr
			requestApp.UpdateCheck.RollbackAllowed = true
			req.Apps = []request.RequestApp{requestApp}

			res, err := request.Send(ctx, req, "M"+vpMilestoneStr+"_rollback")
			if err != nil {
				s.Fatal("Failed to send request: ", err)
			}

			if err := res.ValidateUpdateResponse(); err != nil {
				s.Fatal("Response is not an update: ", err)
			}

			if !res.App.UpdateCheck.Rollback {
				s.Error("Response is not a rollback")
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
