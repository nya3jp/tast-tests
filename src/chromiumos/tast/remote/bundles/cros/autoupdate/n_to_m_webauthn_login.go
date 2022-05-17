// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"math/rand"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/autoupdate/util"
	"chromiumos/tast/rpc"
	webauthnpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NToMWebauthnLogin,
		// We already have lacros variant for normal WebAuthn tests, and cross-version functionality
		// is unrelated specific browser implementation.
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify cross version vault's compatibility",
		Contacts: []string{
			"hcyang@google.com", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"reboot", "chrome", "auto_update_stable"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.hwsec.WebauthnService",
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Params: []testing.Param{{
			Name:              "tpm1",
			ExtraSoftwareDeps: []string{"tpm1"},
		}, {
			Name:              "gsc",
			ExtraSoftwareDeps: []string{"gsc"},
		}},
		Timeout: util.TotalTestTime,
	})
}

func NToMWebauthnLogin(ctx context.Context, s *testing.State) {
	env, err := util.NewHwsecEnv(s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec env: ", err)
	}

	// We need truly random values for username strings so that different test runs don't affect each other.
	rand.Seed(time.Now().UnixNano())

	username := randomUsername()

	ops := &util.Operations{
		PreUpdate: func(ctx context.Context) error {
			return util.ClearTpm(ctx, env)
		},
		PostUpdate: func(ctx context.Context) error {
			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)
			return createUserAndMakeCredential(ctx, env, cl.Conn, username)
		},
		PostRollback: func(ctx context.Context) error {
			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)
			return loginUserAndGetAssertion(ctx, env, cl.Conn, username)
		},
	}

	if err := util.NToMTest(ctx, s.DUT(), s.OutDir(), s.RPCHint(), ops, 3 /*deltaM*/); err != nil {
		s.Fatal("Failed to run cross version test: ", err)
	}
}

func passwordAuth(ctx context.Context, cr webauthnpb.WebauthnServiceClient) error {
	// Type password into ChromeOS WebAuthn dialog.
	if _, err := cr.EnterPassword(ctx, &webauthnpb.EnterPasswordRequest{Password: util.ChromeDefaultPassword}); err != nil {
		return errors.Wrap(err, "failed to type password into ChromeOS auth dialog")
	}
	return nil
}

func createUserAndMakeCredential(ctx context.Context, env *util.HwsecEnv, conn *grpc.ClientConn, username string) error {
	cr := webauthnpb.NewWebauthnServiceClient(conn)

	// Login Chrome and create a WebAuthn credential.
	if _, err := cr.New(ctx, &webauthnpb.NewRequest{
		BrowserType: webauthnpb.BrowserType_ASH,
	}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx, &empty.Empty{})

	if _, err := cr.StartWebauthn(ctx, &webauthnpb.StartWebauthnRequest{
		UserVerification:  webauthnpb.UserVerification_DISCOURAGED,
		AuthenticatorType: webauthnpb.AuthenticatorType_UNSPECIFIED,
		HasDialog:         true,
	}); err != nil {
		return errors.Wrap(err, "failed to start WebAuthn flow")
	}
	defer cr.EndWebauthn(ctx, &empty.Empty{})

	if _, err := cr.StartMakeCredential(ctx, &webauthnpb.StartMakeCredentialRequest{Username: username}); err != nil {
		return errors.Wrap(err, "failed to perform MakeCredential flow")
	}
	if err := passwordAuth(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to perform password auth")
	}
	if _, err := cr.CheckMakeCredential(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to complete MakeCredential")
	}

	return nil
}

func loginUserAndGetAssertion(ctx context.Context, env *util.HwsecEnv, conn *grpc.ClientConn, username string) error {
	cr := webauthnpb.NewWebauthnServiceClient(conn)

	// Login Chrome to see if we can still authenticate the relying party using the WebAuthn credential.
	if _, err := cr.New(ctx, &webauthnpb.NewRequest{KeepState: true}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx, &empty.Empty{})

	if _, err := cr.StartWebauthn(ctx, &webauthnpb.StartWebauthnRequest{
		UserVerification:  webauthnpb.UserVerification_DISCOURAGED,
		AuthenticatorType: webauthnpb.AuthenticatorType_UNSPECIFIED,
		HasDialog:         true,
	}); err != nil {
		return errors.Wrap(err, "failed to start WebAuthn flow")
	}
	defer cr.EndWebauthn(ctx, &empty.Empty{})

	if _, err := cr.StartGetAssertion(ctx, &webauthnpb.StartGetAssertionRequest{Username: username}); err != nil {
		return errors.Wrap(err, "failed to perform GetAssertion flow")
	}
	if err := passwordAuth(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to perform password auth")
	}
	if _, err := cr.CheckGetAssertion(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to complete GetAssertion")
	}

	return nil
}

func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 10)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}
