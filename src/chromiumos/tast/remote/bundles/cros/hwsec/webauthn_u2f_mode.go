// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	webauthnpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebauthnU2fMode,
		Desc: "Checks that WebAuthn under u2f mode succeeds in different configurations",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:firmware", "firmware_cr50"},
		SoftwareDeps: []string{"chrome", "gsc"},
		ServiceDeps: []string{
			"tast.cros.hwsec.WebauthnService",
			"tast.cros.hwsec.AttestationDBusService",
		},
		VarDeps: []string{"servo"},
	})
}

func WebauthnU2fMode(ctx context.Context, s *testing.State) {
	const (
		username = "tast-user@managedchrome.com"
		password = "test0000"
		PIN      = "123456"
	)

	// Create hwsec helper.
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewFullHelper(cmdRunner, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create hwsec remote helper: ", err)
	}

	// Ensure TPM is ready before running the tests.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure TPM is ready: ", err)
	}

	// Connect to the chrome service server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// u2fd reads files from the user's home dir, so we need to log in.
	cr := webauthnpb.NewWebauthnServiceClient(cl.Conn)
	if _, err := cr.New(ctx, &webauthnpb.NewRequest{
		Username: username,
		Password: password,
	}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx, &empty.Empty{})

	if _, err := cr.SetupPin(ctx, &webauthnpb.SetupPinRequest{Pin: PIN}); err != nil {
		s.Fatal("Failed to setup PIN: ", err)
	}

	// Ensure TPM is prepared for enrollment.
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	chaps, err := pkcs11.NewChaps(ctx, cmdRunner, helper.CryptohomeClient())
	if err != nil {
		s.Fatal("Failed to create chaps client: ", err)
	}

	// Ensure chaps finished the initialization.
	// U2F didn't depend on chaps, but chaps would block the TPM operations, and caused U2F timeout.
	if err := util.EnsureChapsSlotsInitialized(ctx, chaps); err != nil {
		s.Fatal("Failed to ensure chaps slots: ", err)
	}

	// Connect to servo.
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()
	// Set u2f mode to enable power button press authentication.
	util.SetU2fdFlags(ctx, helper, true, true, false)
	// Clean up the flags in u2fd after the tests finished.
	defer util.SetU2fdFlags(ctx, helper, false, false, false)

	pinAuthCallback := func(ctx context.Context) error {
		// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
		if _, err := cr.EnterPin(ctx, &webauthnpb.EnterPinRequest{Pin: PIN}); err != nil {
			return errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
		}
		return nil
	}
	powerButtonAuthCallback := func(ctx context.Context) error {
		// Press power button using servo.
		if err := svo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "failed to press the power key")
		}
		return nil
	}

	for _, tc := range []struct {
		name              string
		userVerification  webauthnpb.UserVerification
		authenticatorType webauthnpb.AuthenticatorType
		hasDialog         bool
		authCallback      func(ctx context.Context) error
	}{
		{
			name:              "discouraged_unspecified",
			userVerification:  webauthnpb.UserVerification_DISCOURAGED,
			authenticatorType: webauthnpb.AuthenticatorType_UNSPECIFIED,
			hasDialog:         false,
			authCallback:      powerButtonAuthCallback,
		},
		{
			name:              "discouraged_cross_plaform",
			userVerification:  webauthnpb.UserVerification_DISCOURAGED,
			authenticatorType: webauthnpb.AuthenticatorType_CROSS_PLATFORM,
			hasDialog:         false,
			authCallback:      powerButtonAuthCallback,
		},
		{
			name: "discouraged_platform",

			userVerification:  webauthnpb.UserVerification_DISCOURAGED,
			authenticatorType: webauthnpb.AuthenticatorType_PLATFORM,
			hasDialog:         false,
			authCallback:      powerButtonAuthCallback,
		},
		{
			name: "preferred_unspecified",

			userVerification:  webauthnpb.UserVerification_PREFERRED,
			authenticatorType: webauthnpb.AuthenticatorType_UNSPECIFIED,
			hasDialog:         true,
			authCallback:      pinAuthCallback,
		},
		{
			name: "preferred_cross_plaform",

			userVerification:  webauthnpb.UserVerification_PREFERRED,
			authenticatorType: webauthnpb.AuthenticatorType_CROSS_PLATFORM,
			hasDialog:         false,
			authCallback:      powerButtonAuthCallback,
		},
		{
			name: "preferred_platform",

			userVerification:  webauthnpb.UserVerification_PREFERRED,
			authenticatorType: webauthnpb.AuthenticatorType_PLATFORM,
			hasDialog:         true,
			authCallback:      pinAuthCallback,
		},
		{
			name: "required_unspecified",

			userVerification:  webauthnpb.UserVerification_REQUIRED,
			authenticatorType: webauthnpb.AuthenticatorType_UNSPECIFIED,
			hasDialog:         true,
			authCallback:      pinAuthCallback,
		},
		{
			name: "required_platform",

			userVerification:  webauthnpb.UserVerification_REQUIRED,
			authenticatorType: webauthnpb.AuthenticatorType_PLATFORM,
			hasDialog:         true,
			authCallback:      pinAuthCallback,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if _, err := cr.StartWebauthn(ctx, &webauthnpb.StartWebauthnRequest{
				UserVerification:  tc.userVerification,
				AuthenticatorType: tc.authenticatorType,
				HasDialog:         tc.hasDialog,
			}); err != nil {
				s.Fatal("Failed to start WebAuthn flow: ", err)
			}
			defer cr.EndWebauthn(ctx, &empty.Empty{})

			if _, err := cr.StartMakeCredential(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to perform MakeCredential flow: ", err)
			}
			if err := tc.authCallback(ctx); err != nil {
				s.Fatal("Failed to call the auth callback: ", err)
			}
			if _, err := cr.CheckMakeCredential(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to complete MakeCredential: ", err)
			}
			if _, err := cr.StartGetAssertion(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to perform GetAssertion flow: ", err)
			}
			if err := tc.authCallback(ctx); err != nil {
				s.Fatal("Failed to call the auth callback: ", err)
			}
			if _, err := cr.CheckGetAssertion(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to complete GetAssertion: ", err)
			}
		})
	}

}
