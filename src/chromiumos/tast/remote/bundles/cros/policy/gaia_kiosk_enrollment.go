// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/gaiaenrollment"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ks "chromiumos/tast/services/cros/kiosk"
	ps "chromiumos/tast/services/cros/policy"
	ts "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

const gaiaKioskEnrollmentTimeout = 7 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAKioskEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a kiosk device and make sure kiosk app started",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.kiosk.KioskService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      gaiaKioskEnrollmentTimeout,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: gaiaenrollment.TestParams{
					DMServer: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					PoolID:   tape.EnrollmentKiosk,
				},
			},
		},
		Vars: []string{
			tape.ServiceAccountVar,
		},
	})
}

func GAIAKioskEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(gaiaenrollment.TestParams)
	dmServerURL := param.DMServer
	poolID := param.PoolID

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(cleanupCtx)

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	pc := ps.NewPolicyServiceClient(cl.Conn)
	kc := ks.NewKioskServiceClient(cl.Conn)

	kioskErr := make(chan error)
	checkKioskStarted := func() {
		kioskErr <- func() error {
			if _, err := kc.ConfirmKioskStarted(ctx, &ks.ConfirmKioskStartedRequest{}); err != nil {
				return errors.Wrap(err, "failed to start kiosk mode")
			}
			return nil
		}()
	}

	go checkKioskStarted()

	tapeClient, err := tape.NewClient(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}

	timeout := int32(gaiaKioskEnrollmentTimeout.Seconds())
	// Create an account manager and lease a test account for the duration of the test.
	accManager, acc, err := tape.NewOwnedTestAccountManagerFromClient(ctx, tapeClient /*lock*/, false, tape.WithTimeout(timeout), tape.WithPoolID(poolID))
	if err != nil {
		s.Fatal("Failed to create an account manager and lease an account: ", err)
	}
	defer accManager.CleanUp(cleanupCtx)

	if _, err := pc.GAIAEnrollUsingChrome(ctx, &ps.GAIAEnrollUsingChromeRequest{
		Username:    acc.Username,
		Password:    acc.Password,
		DmserverURL: dmServerURL,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	// Deprovision the DUT at the end of the test.
	defer func(ctx context.Context) {
		tapeService := ts.NewServiceClient(cl.Conn)
		// Get the device ID of the DUT to deprovision it at the end of the test.
		res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: acc.CustomerID})
		if err != nil {
			s.Fatal("Failed to get the deviceID: ", err)
		}
		if err = tapeClient.Deprovision(ctx, res.DeviceID, acc.CustomerID); err != nil {
			s.Fatalf("Failed to deprovision device %s: %v", res.DeviceID, err)
		}
	}(cleanupCtx)

	if err := <-kioskErr; err != nil {
		s.Error("kiosk failed to start: ", err)
	}
}
