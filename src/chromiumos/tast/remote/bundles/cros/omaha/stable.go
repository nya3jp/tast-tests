// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package omaha

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/omaha/request"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Stable,
		Desc: "Check if the current stable",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:omaha"},
		SoftwareDeps: []string{},
	})
}

/*

<request requestid="1c680322-6097-4da0-9b40-ef55fbe1bf7e" sessionid="0e2c513c-6a9e-4c0d-8c13-46a6a2e76d65" protocol="3.0" updater="ChromeOSUpdateEngine" updaterversion="0.1.0.0" installsource="scheduler" ismachine="1">
    <os version="Indy" platform="Chrome OS" sp="13729.72.0_x86_64"></os>
    <app appid="{01906EA2-3EB2-41F1-8F62-F0B7120EFD2E}" cohort="1:35:44@0.05" cohortname="eve_eve_stable" version="13729.72.0" track="stable-channel" board="eve-signed-mp-v2keys" hardware_class="EVE D6C-A4E-D4H-F8N-P8A-A53" delta_okay="true" installdate="4837" lang="en-US" >
        <event eventtype="14" eventresult="1"></event>
    </app>
</request>

*/

func Stable(ctx context.Context, s *testing.State) {
	const prevVersion = "13421.53.0"

	req := request.New()

	params, err := request.LoadParamsFromDUT(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to load device parameters: ", err)
	}

	s.Log("Device parameters: ", params)

	req.OS.SP = params.GenSP(prevVersion)
	app := params.GenAPPRequest(prevVersion, request.Stable)
	// app.Board = "sarien-signed-123"
	req.Apps = append(req.Apps, app)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := request.Send(ctx, req)
	if err != nil {
		s.Fatal("Failed to send request: ", err)
	}

	s.Log("Response: ", res)
}
