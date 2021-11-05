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

	dutParams, err := request.LoadParamsFromDUT(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to load device parameters: ", err)
	}
	if err := dutParams.DumpToFile(filepath.Join(s.OutDir(), "device-param.json")); err != nil {
		s.Log("Failed to dump 'device-param.json': ", err)
	}

	req := request.New()
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

	if err := res.ValidateUpdateResponse(); err != nil {
		s.Fatal("Response is not an update: ", err)
	}

	chromeOSVersion, err := res.ChromeOSVersion()
	if err != nil {
		s.Error("Failed to get ChromeOS version: ", err)
	}
	if !request.MatchOneOfVersions(chromeOSVersion, params.CurrentChromeOSLTS) {
		s.Errorf("ChromeOS Version %q does not match the requested prefix %q", chromeOSVersion, params.CurrentChromeOSLTS)
	}

	currentChromeOSLTSMinor, err := strconv.Atoi(params.CurrentChromeOSLTSMinor)
	if err != nil {
		s.Fatalf("Could not parse LTS minor version %q to int: %v", params.CurrentChromeOSLTSMinor, err)
	}

	minorVersion, err := strconv.Atoi(strings.Split(chromeOSVersion, ".")[1])
	if err != nil {
		s.Fatalf("Failed to read the minor version from %q: %v", chromeOSVersion, err)
	}

	if minorVersion < currentChromeOSLTSMinor {
		s.Errorf("Minor version %d not an LTS minor version (>=%d)", minorVersion, params.CurrentChromeOSLTSMinor)
	}
}
