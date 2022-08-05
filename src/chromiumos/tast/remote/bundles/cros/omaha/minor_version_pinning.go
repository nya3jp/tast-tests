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
		Func: MinorVersionPinning,
		Desc: "Check if an appropriate versions are served when pinning to a minor version",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{"auto_update_stable"},
		Fixture:      fixture.Omaha,
	})
}

func MinorVersionPinning(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	req := request.New()
	req.GenSP(state.Device, state.Config.OldVersion)
	requestApp := request.GenerateRequestApp(state.Device, state.Config.OldVersion, request.Stable)

	for _, installSource := range []string{request.InstallSourceScheduler, request.InstallSourceOnDemand} {
		s.Run(ctx, "install_source_"+installSource, func(ctx context.Context, s *testing.State) {

			for _, pin := range state.MinorVersionPins {
				if pin.Board != state.Device.RawBoard {
					// Skip pins for other boards.
					continue
				}

				selectorString := strconv.FormatInt(int64(pin.Selector), 10)

				s.Run(ctx, "selector_"+selectorString, func(ctx context.Context, s *testing.State) {
					ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()

					// requestApp.UpdateCheck.TargetVersionPrefix = "14909"
					requestApp.UpdateCheck.TargetVersionSelector = selectorString
					req.Apps = []request.RequestApp{requestApp}
					req.InstallSource = installSource

					res, err := request.Send(ctx, req, "S-"+selectorString)
					if err != nil {
						s.Fatal("Failed to send request: ", err)
					}

					if err := res.ValidateUpdateResponse(); err != nil {
						s.Fatal("Response is not an update: ", err)
					}

					expectedVersion := func(version string) bool {
						for _, v := range pin.Versions {
							if v == version {
								return true
							}
						}

						return false
					}

					if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
						s.Error("Failed to get ChromeOS version: ", err)
					} else if !expectedVersion(chromeOSVersion) {
						s.Errorf("ChromeOS Version %q does not match the expected versions %v", chromeOSVersion, pin.Versions)
					} else {
						s.Logf("Selector %s -> %s", selectorString, chromeOSVersion)
					}
				})
			}
		})

		s.Run(ctx, "expired", func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// requestApp.UpdateCheck.TargetVersionPrefix = selectorString
			requestApp.UpdateCheck.TargetVersionSelector = "1000" // Expired in 1970.
			req.Apps = []request.RequestApp{requestApp}
			req.InstallSource = installSource

			res, err := request.Send(ctx, req, "expired")
			if err != nil {
				s.Fatal("Failed to send request: ", err)
			}

			if err := res.ValidateUpdateResponse(); err == nil {
				s.Error("Got an update for an expired pin")

				if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
					s.Error("Failed to get version: ", err)
				} else {
					s.Error("Got an update to version: ", chromeOSVersion)
				}

			}
		})
	}
}
