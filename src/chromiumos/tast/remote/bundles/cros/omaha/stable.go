// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/remote/bundles/cros/omaha/params"
	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Stable,
		Desc: "Check if an appropriate stable version is being served",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{},
	})
}

func Stable(ctx context.Context, s *testing.State) {
	// This is an old version. No need to update it.
	const prevVersion = "13421.53.0"

	req := request.New()

	dutParams, err := request.LoadParamsFromDUT(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to load device parameters: ", err)
	}
	if err := dutParams.DumpToFile(filepath.Join(s.OutDir(), "device-param.json")); err != nil {
		s.Log("Failed to dump 'device-param.json': ", err)
	}

	req.OS.SP = dutParams.GenSP(prevVersion)
	app := dutParams.GenRequestApp(prevVersion, request.Stable)
	req.Apps = append(req.Apps, app)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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
	} else if !request.MatchOneOfVersions(chromeVersion, params.CurrentStableChrome, params.NextStableChrome) {
		s.Errorf("Chrome Version %q does not match the current %q or next %q milestone", chromeVersion, params.CurrentStableChrome, params.NextStableChrome)
	}

	if chromeOSVersion, err := res.ChromeOSVersion(); err != nil {
		s.Error("Failed to get ChromeOS version: ", err)
	} else if !request.MatchOneOfVersions(chromeOSVersion, params.CurrentStableChromeOS, params.NextStableChromeOS) {
		s.Errorf("ChromeOS Version %q does not match the current %q or next %q milestone", chromeOSVersion, params.CurrentStableChromeOS, params.NextStableChromeOS)
	}
}
