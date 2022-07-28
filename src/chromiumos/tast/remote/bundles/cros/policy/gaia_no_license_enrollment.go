// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testResources struct {
	username string // username for Chrome login
	password string // password to login
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIANoLicenseEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Try to GAIA Enroll a device to a domain with no licenses; confirm error happens",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
		},
		Timeout: 7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testResources{
					username: "policy.GAIANoLicenseEnrollment.user_name",
					password: "policy.GAIANoLicenseEnrollment.password",
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{
			"policy.GAIANoLicenseEnrollment.user_name",
			"policy.GAIANoLicenseEnrollment.password",
		},
	})
}

func GAIANoLicenseEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testResources)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	policyClient := pspb.NewPolicyServiceClient(cl.Conn)

	if _, err := policyClient.StartNewChromeReader(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to initialize syslog reader: ", err)
	}

	if _, err := policyClient.GAIAEnrollUsingChrome(ctx, &pspb.GAIAEnrollUsingChromeRequest{
		Username:    username,
		Password:    password,
		DmserverURL: dmServerURL,
	}); err == nil {
		s.Fatal("Enrollment was successful and it shouldn't have been: ", err)
	}

	if _, err := policyClient.WaitForEnrollmentError(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to get error message: ", err)
	}

}
