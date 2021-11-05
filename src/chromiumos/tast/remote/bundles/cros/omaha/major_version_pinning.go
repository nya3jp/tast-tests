// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

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
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{},
	})
}

func MajorVersionPinning(ctx context.Context, s *testing.State) {
	// This is an old version. No need to update it unless a new stepping stone is introduced.
	// This is the latest M86 stable push.
	const prevVersion = "13421.102.0"

	milestoneToCROSVersion := map[int]int{
		84: 13099,
		85: 13310,
		86: 13421,
		87: 13505,
		88: 13597,
		89: 13729,
		90: 13816,
		91: 13904,
		92: 13982,
		94: 14150,
		96: 14268,
	}

	req := request.New()

	dutParams, err := request.LoadParamsFromDUT(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to load device parameters: ", err)
	}
	if err := dutParams.DumpToFile(filepath.Join(s.OutDir(), "device-param.json")); err != nil {
		s.Log("Failed to dump 'device-param.json': ", err)
	}

	req.GenSP(dutParams, prevVersion)
	req.AddRequestApp(dutParams, prevVersion, request.Stable)

	for vpChromeVersion, vpChromeOSVersion := range milestoneToCROSVersion {
		vpChromeVersionStr := strconv.FormatInt(int64(vpChromeVersion), 10)
		vpChromeOSVersionStr := strconv.FormatInt(int64(vpChromeOSVersion), 10)

		s.Run(ctx, "M"+vpChromeVersionStr, func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req.Apps[0].UpdateCheck.TargetVersionPrefix = vpChromeOSVersionStr

			res, err := request.Send(ctx, req, "stable")
			if err != nil {
				s.Fatal("Failed to send request: ", err)
			}

			if res.Server != "prod" {
				s.Errorf("Reached wrong server: got %q; want %q", res.Server, "prod")
			}

			if res.App.Status != "ok" {
				s.Errorf("Unexpected App status: got %q; want %q", res.App.Status, "ok")
			}

			if res.App.UpdateCheck.Status != "ok" {
				s.Errorf("Unexpected UpdateCheck status: got %q; want %q", res.App.UpdateCheck.Status, "ok")
			}

			// Check if the current stable is being served. We also accept the next stable
			// here to not break during the transition from Mxx to Mxx+1.
			if chromeVersion, err := res.ChromeVersion(); err != nil {
				s.Error("Failed to get Chrome version: ", err)
			} else if !request.MatchOneOfVersions(chromeVersion, vpChromeVersionStr) {
				s.Errorf("Chrome Version %q does not match the expected milestone %d", chromeVersion, vpChromeVersion)
			}

			if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
				s.Error("Failed to get ChromeOS version: ", err)
			} else if !request.MatchOneOfVersions(chromeOSVersion, vpChromeOSVersionStr) {
				s.Errorf("ChromeOS Version %q does not match the expected prefix %d", chromeOSVersion, vpChromeOSVersion)
			}
		})
	}
}
