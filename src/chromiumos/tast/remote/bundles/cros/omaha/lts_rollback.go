// Copyright 2022 The ChromiumOS Authors.
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
		Func: LTSRollback,
		Desc: "Check if an appropriate LTS version is being served for rollback",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{"auto_update_stable"},
		Fixture:      fixture.Omaha,
		Timeout:      30 * time.Second,
	})
}

func LTSRollback(ctx context.Context, s *testing.State) {
	state := s.FixtValue().(*params.FixtData)

	chromeOSStableInt := state.Config.ChromeOSVersionFromMilestone[state.Config.NextStableChrome]
	chromeOSStable := strconv.FormatInt(int64(chromeOSStableInt), 10) + ".0.0"

	req := request.New()
	req.GenSP(state.Device, chromeOSStable)

	ltsChromeOsVersion := state.Config.ChromeOSVersionFromMilestone[state.Config.CurrentChromeOSLTS]
	ltsPrefix := strconv.FormatInt(int64(ltsChromeOsVersion), 10)

	requestApp := request.GenerateRequestApp(state.Device, chromeOSStable, request.Stable)
	requestApp.UpdateCheck.LTSTag = "lts"
	requestApp.UpdateCheck.TargetVersionPrefix = ltsPrefix
	requestApp.UpdateCheck.RollbackAllowed = true
	req.Apps = append(req.Apps, requestApp)

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

	if minorVersion < state.Config.CurrentChromeOSLTSMinor {
		s.Errorf("Minor version %d not an LTS minor version (>=%d)", minorVersion, state.Config.CurrentChromeOSLTSMinor)
	}
}
