// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/remote/bundles/cros/omaha/params"
	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LTS,
		Desc: "Check if an appropriate LTS version is being served",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{},
	})
}

func LTS(ctx context.Context, s *testing.State) {
	// This is an old version. No need to update it unless a new stepping stone is introduced.
	// This is the latest M86 stable push.
	const prevVersion = "13421.102.0"

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
	req.Apps[0].UpdateCheck.LTSTag = "lts"
	req.Apps[0].UpdateCheck.TargetVersionPrefix = params.CurrentChromeOSLTS

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := request.Send(ctx, req, "lts")
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

	chromeOSVersion, err := res.ChromeOSVersion()
	if err != nil {
		s.Error("Failed to get ChromeOS version: ", err)
	}
	if !request.MatchOneOfVersions(chromeOSVersion, params.CurrentChromeOSLTS) {
		s.Errorf("ChromeOS Version %q does not match the requested prefix %q", chromeOSVersion, params.CurrentChromeOSLTS)
	}

	minorVersion, err := strconv.Atoi(strings.Split(chromeOSVersion, ".")[1])
	if err != nil {
		s.Fatalf("Failed to read the minor version from %q: %v", chromeOSVersion, err)
	}
	if minorVersion < params.CurrentChromeOSLTSMinor {
		s.Errorf("Minor version %d not an LTS minor version (>=%d)", minorVersion, params.CurrentChromeOSLTSMinor)
	}
}
