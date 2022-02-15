// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Pkcs11ChangeAuthData,
		Desc: "Verifies pkcs11 behavior act as expected after change authorization data",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 4 * time.Minute,
	})
}

// Pkcs11ChangeAuthData test the chapsd behavior of change auth data
func Pkcs11ChangeAuthData(ctx context.Context, s *testing.State) {
	r := libhwseclocal.NewCmdRunner()

	scratchpadPath := "/tmp/chaps/"
	slot, err := pkcs11test.SetupP11TestToken(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}

	cleanupCtx := ctx
	//Give cleanup function 20 seconds to remove scratchpad
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth1")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--replay_wifi"); err != nil {
		s.Error("Failed to load test token (1): ", err)
	}

	// Change auth data while the token is not loaded.
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth1", "auth2")
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth2")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (1): ", err)
	}

	// Change auth data while the token is loaded.
	pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth2", "auth3")
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (2): ", err)
	}

	// Attempt change with incorrect current auth data.
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "bad_auth", "auth4")
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (3): ", err)
	}

	// Verify old auth data no longer works after change. This also verifies
	// recovery from bad auth data - expect a functional, empty token.
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth3", "auth5")
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (1): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--replay_wifi"); err != nil {
		s.Error("Failed to load test token (2): ", err)
	}
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)

	// Token should have been recreated with 'auth3'.
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Token not valid after recovery: ", err)
	}
	pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath)

	// Since token was recovered, previous correct auth should be rejected.
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth5")
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (2): ", err)
	}

	// Token should be invalid after remove scratchpad
	pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath)
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err == nil {
		s.Error("Bad authorization data allowed (3): ", err)
	}
}
