// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	tspb "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

type testInfo struct {
	dmserver string // device management server url
	poolID   string // poolID for the used test account
}

const gaiaEnrollmentTimeout = 7 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device without checking policies",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.tape.Service"},
		Timeout:      gaiaEnrollmentTimeout,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testInfo{
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					poolID:   tape.Enrollment,
				},
			},
			{
				Name: "autopush_new_saml",
				Val: testInfo{
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					poolID:   tape.EnrollmentSaml,
				},
			},
		},
		Vars: []string{
			tape.ServiceAccountVar,
		},
	})
}

func GAIAEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testInfo)
	dmServerURL := param.dmserver
	poolID := param.poolID

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(cleanupCtx)

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	pc := pspb.NewPolicyServiceClient(cl.Conn)

	tapeClient, err := tape.NewClient(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}

	// Create an account manager and lease a test account for the duration of the test.
	accManager, acc, err := tape.NewOwnedTestAccountManagerFromClient(ctx, tapeClient, false, tape.WithTimeout(int32(gaiaEnrollmentTimeout.Seconds())), tape.WithPoolID(poolID))
	if err != nil {
		s.Fatal("Failed to create an account manager and lease an account: ", err)
	}
	defer accManager.CleanUp(cleanupCtx)

	if _, err := pc.GAIAEnrollUsingChrome(ctx, &pspb.GAIAEnrollUsingChromeRequest{
		Username:    acc.Username,
		Password:    acc.Password,
		DmserverURL: dmServerURL,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	tapeService := tspb.NewServiceClient(cl.Conn)
	// Get the device id of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &tspb.GetDeviceIDRequest{CustomerID: acc.CustomerID})
	if err != nil {
		s.Fatal("Failed to get the deviceID: ", err)
	}

	// Deprovision the DUT at the end of the test.
	defer func(ctx context.Context) {
		if err = tapeClient.Deprovision(ctx, res.DeviceID, acc.CustomerID); err != nil {
			s.Fatalf("Failed to deprovision device %s: %v", res.DeviceID, err)
		}
	}(cleanupCtx)
}
