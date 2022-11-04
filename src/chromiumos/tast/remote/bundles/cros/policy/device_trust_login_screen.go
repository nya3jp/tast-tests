// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/services/cros/enterpriseconnectors"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/testing"
)

const deviceTrustEnrollmentTimeout = 7 * time.Minute

type userParam struct {
	poolID        string
	loginPossible bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceTrustLoginScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Device Trust is working on login screen with a fake IdP",
		Contacts: []string{
			"lmasopust@google.com",
			"rodmartin@google.com",
			"cbe-device-trust-eng@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"reboot",
		},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.tape.Service",
			"tast.cros.enterpriseconnectors.DeviceTrustService",
			"tast.cros.graphics.ScreenshotService",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
			tape.ServiceAccountVar,
		},
		Params: []testing.Param{{
			Name: "host_allowed",
			Val: userParam{
				poolID:        tape.DeviceTrustEnabled,
				loginPossible: true,
			},
		}, {
			Name: "host_not_allowed",
			Val: userParam{
				poolID:        tape.DeviceTrustDisabled,
				loginPossible: false,
			},
		}},
		Timeout: 7 * time.Minute,
	})
}

func DeviceTrustLoginScreen(ctx context.Context, s *testing.State) {
	param := s.Param().(userParam)
	poolID := param.poolID

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

	tapeClient, err := tape.NewClient(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}

	timeout := int32(deviceTrustEnrollmentTimeout.Seconds())
	// Create an account manager and lease a test account for the duration of the test.
	accManager, acc, err := tape.NewOwnedTestAccountManagerFromClient(ctx, tapeClient, false /*lock*/, tape.WithTimeout(timeout), tape.WithPoolID(poolID))
	if err != nil {
		s.Fatal("Failed to create an account manager and lease an account: ", err)
	}
	defer accManager.CleanUp(cleanupCtx)

	screenshotService := graphics.NewScreenshotServiceClient(cl.Conn)
	captureScreenshotOnError := func(ctx context.Context, hasError func() bool) {
		if !hasError() {
			return
		}

		screenshotService.CaptureScreenshot(ctx, &graphics.CaptureScreenshotRequest{FilePrefix: "deviceTrustLoginError"})
	}
	defer captureScreenshotOnError(ctx, s.HasError)

	service := enterpriseconnectors.NewDeviceTrustServiceClient(cl.Conn)
	s.Log("Enrolling device")
	if _, err = service.Enroll(ctx, &enterpriseconnectors.EnrollRequest{User: acc.Username, Pass: acc.Password}); err != nil {
		s.Fatal("Remote call Enroll() failed: ", err)
	}

	// Deprovision the DUT at the end of the test.
	defer func(ctx context.Context) {
		if err := tapeClient.DeprovisionHelper(cleanupCtx, cl, acc.CustomerID); err != nil {
			s.Fatal("Failed to deprovision device: ", err)
		}
	}(cleanupCtx)

	res, err := service.LoginWithFakeIdP(ctx, &enterpriseconnectors.LoginWithFakeIdPRequest{SigninProfileTestExtensionManifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey")})
	if err != nil {
		s.Fatal("Remote call LoginWithFakeIdP() failed: ", err)
	}

	if res.Succesful != param.loginPossible {
		s.Errorf("Unexpected value for loginPossible: got %t, want %t", res.Succesful, param.loginPossible)
	}
}
