// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/remote/bundles/cros/omaha/params"
	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LTSPinning,
		Desc: "Check if an appropriate LTS version is being served when pinned to older releases",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{"auto_update_stable"},
		Fixture:      fixture.Omaha,
	})
}

func LTSPinning(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	for chromeOsLTS, chromeOSLTSMinor := range state.Config.ChromeOSLTRMilestoneWithMinimumMinor {
		chromeOsLTSStr := strconv.FormatInt(int64(chromeOsLTS), 10)

		s.Run(ctx, "M"+chromeOsLTSStr, func(ctx context.Context, s *testing.State) {
			ltsChromeOsVersion := state.Config.ChromeOSVersionFromMilestone[chromeOsLTS]
			ltsPrefix := strconv.FormatInt(int64(ltsChromeOsVersion), 10)

			prevVersion, err := state.Config.PreviousMilestoneOSVersion(chromeOsLTS)
			if err != nil {
				s.Fatal("Failed to get previous version: ", err)
			}

			req := request.New()
			req.GenSP(state.Device, prevVersion)

			requestApp := request.GenerateRequestApp(state.Device, prevVersion, request.Stable)
			requestApp.UpdateCheck.LTSTag = "lts"
			requestApp.UpdateCheck.TargetVersionPrefix = ltsPrefix
			req.Apps = append(req.Apps, requestApp)

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			res, err := request.Send(ctx, req, "lts")
			if err != nil {
				s.Fatal("Failed to send request: ", err)
			}

			if err := res.ValidateUpdateResponse(); err != nil {
				s.Fatal("Response is not an update: ", err)
			}

			chromeOSVersion, err := res.ChromeOSVersion()
			if err != nil {
				s.Error("Failed to get ChromeOS version: ", err)
			}
			if !request.MatchOneOfVersions(chromeOSVersion, ltsChromeOsVersion) {
				s.Errorf("ChromeOS Version %q does not match the requested prefix %q", chromeOSVersion, ltsPrefix)
			}

			minorVersion, err := strconv.Atoi(strings.Split(chromeOSVersion, ".")[1])
			if err != nil {
				s.Fatalf("Failed to read the minor version from %q: %v", chromeOSVersion, err)
			}

			if minorVersion < chromeOSLTSMinor {
				s.Errorf("Unexpected LTS version; got %s, expected minor >= %d", chromeOSVersion, chromeOSLTSMinor)
			}
		})
	}
}
