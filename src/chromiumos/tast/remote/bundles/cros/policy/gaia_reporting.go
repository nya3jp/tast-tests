// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testParameters struct {
	username             string // username for Chrome login
	password             string // password to login
	dmserver             string // device management server url
	reportingserver      string // reporting api server url
	obfuscatedcustomerid string // external customer id
	debugservicekey      string // debug service key
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify reporting functionality",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"cros-reporting-team@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.ReportingService"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testParameters{
					username:             "policy.GAIAReporting.user_name",
					password:             "policy.GAIAReporting.password",
					dmserver:             "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					reportingserver:      "https://autopush-chromereporting-pa.sandbox.googleapis.com/v1",
					obfuscatedcustomerid: "policy.GAIAReporting.obfuscated_customer_id",
					debugservicekey:      "policy.GAIAReporting.lookup_events_api_key",
				},
			},
		},
		Vars: []string{
			"policy.GAIAReporting.user_name",
			"policy.GAIAReporting.password",
			"policy.GAIAReporting.obfuscated_customer_id",
			"policy.GAIAReporting.lookup_events_api_key",
		},
	})
}

func GAIAReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(testParameters)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver
	reportingServerURL := param.reportingserver
	obfuscatedCustomerID := s.RequiredVar(param.obfuscatedcustomerid)
	debugServiceAPIKey := s.RequiredVar(param.debugservicekey)

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewReportingServiceClient(cl.Conn)

	if _, err := pc.GAIAEnrollUsingChromeAndCollectReporting(ctx, &ps.GAIAEnrollUsingChromeAndCollectReportingRequest{
		Username:             username,
		Password:             password,
		DmserverURL:          dmServerURL,
		ReportingserverURL:   reportingServerURL,
		ObfuscatedCustomerID: obfuscatedCustomerID,
		DebugServiceAPIKey:   debugServiceAPIKey,
	}); err != nil {
		s.Error("Failed to enroll or collect reporting using chrome: ", err)
	}
}
