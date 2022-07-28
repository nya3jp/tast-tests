// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testDetails struct {
	username string // username for Chrome login
	password string // password to login
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAFlexorgsEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device to FlexOrgs domain without checking policies",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush_flexorgs",
				Val: testDetails{
					username: "policy.GAIAFlexorgsEnrollment.flex_user_name",
					password: "policy.GAIAFlexorgsEnrollment.flex_password",
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{
			"policy.GAIAFlexorgsEnrollment.flex_user_name",
			"policy.GAIAFlexorgsEnrollment.flex_password",
		},
	})
}

func GAIAFlexorgsEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testDetails)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	policyClient := pspb.NewPolicyServiceClient(cl.Conn)

	if _, err := policyClient.GAIAEnrollUsingChrome(ctx, &pspb.GAIAEnrollUsingChromeRequest{
		Username:    username,
		Password:    password,
		DmserverURL: dmServerURL,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
}
