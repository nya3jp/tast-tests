// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ks "chromiumos/tast/services/cros/kiosk"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testCreds struct {
	username string // username for Chrome login
	password string // password to login
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAKioskEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a kiosk device and make sure kiosk app started",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{}, // Need dedicated device ready in the lab, b/227604028.
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.kiosk.KioskService", "tast.cros.hwsec.OwnershipService"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testCreds{
					username: "policy.GAIAKioskEnrollment.user_name",
					password: "policy.GAIAKioskEnrollment.password",
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{
			"policy.GAIAKioskEnrollment.user_name",
			"policy.GAIAKioskEnrollment.password",
		},
	})
}

func GAIAKioskEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testCreds)
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

	if _, err := pc.GAIAEnrollUsingChrome(ctx, &ps.GAIAEnrollUsingChromeRequest{
		Username:    username,
		Password:    password,
		DmserverURL: dmServerURL,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	if err := <-kioskErr; err != nil {
		s.Error("kiosk failed to start: ", err)
	}
}
