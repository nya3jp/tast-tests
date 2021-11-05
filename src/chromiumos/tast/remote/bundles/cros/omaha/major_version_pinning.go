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
	params := s.FixtValue().(*params.FixtData)

	req := request.New()
	req.GenSP(params.Device, params.Config.OldVersion)
	requestApp := request.GenerateRequestApp(params.Device, params.Config.OldVersion, request.Stable)

	for vpChromeVersion, vpChromeOSVersion := range params.Config.ChromeOSVersionFromMilestone {
		if vpChromeVersion > params.Config.CurrentStableChrome {
			s.Logf("Not testing version %d higher than stable %d", vpChromeVersion, params.Config.CurrentStableChrome)
			continue
		}

		vpChromeVersionStr := strconv.FormatInt(int64(vpChromeVersion), 10)
		vpChromeOSVersionStr := strconv.FormatInt(int64(vpChromeOSVersion), 10)

		s.Run(ctx, "M"+vpChromeVersionStr, func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			requestApp.UpdateCheck.TargetVersionPrefix = vpChromeOSVersionStr
			req.Apps = []request.RequestApp{requestApp}

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
			} else if !request.MatchOneOfVersions(chromeVersion, vpChromeVersion) {
				s.Errorf("Chrome Version %q does not match the expected milestone %d", chromeVersion, vpChromeVersion)
			}

			if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
				s.Error("Failed to get ChromeOS version: ", err)
			} else if !request.MatchOneOfVersions(chromeOSVersion, vpChromeOSVersion) {
				s.Errorf("ChromeOS Version %q does not match the expected prefix %d", chromeOSVersion, vpChromeOSVersion)
			}
		})
	}
}
