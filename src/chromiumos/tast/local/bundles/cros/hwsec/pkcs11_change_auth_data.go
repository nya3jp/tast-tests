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

// Pkcs11ChangeAuthData test the chapsd behavior of change auth data.
func Pkcs11ChangeAuthData(ctx context.Context, s *testing.State) {
	r := libhwseclocal.NewCmdRunner()

	scratchpadPath := "/tmp/chaps/"
	if err := pkcs11test.SetupP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}

	cleanupCtx := ctx
	// Give cleanup function 20 seconds to remove scratchpad.
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	// Test 1: Check token load successfully.
	slot, err := pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth1")
	if err != nil {
		s.Error("Failed to load token using chaps_client (1): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--replay_wifi"); err != nil {
		s.Error("Load token failed (1): ", err)
	}

	// Test 2: Change auth data while the token is not loaded.
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (1): ", err)
	}
	if err := pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth1", "auth2"); err != nil {
		s.Error("Failed to change token using chaps_client (1): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth2")
	if err != nil {
		s.Error("Failed to load token using chaps_client (2): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (1): ", err)
	}

	// Test 3: Change auth data while the token is loaded.
	if err := pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth2", "auth3"); err != nil {
		s.Error("Failed to change token using chaps_client (2): ", err)
	}
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (2): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if err != nil {
		s.Error("Failed to load token using chaps_client (3): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (2): ", err)
	}

	// Test 4: Attempt change auth data with incorrect current auth data.
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (3): ", err)
	}
	if err = pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "bad_auth", "auth4"); err != nil {
		s.Error("Failed to change token using chaps_client (3): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if err != nil {
		s.Error("Failed to load token using chaps_client (4): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (3): ", err)
	}

	// Test 5: Verify old auth data no longer works after changed. This also verifies
	// recovery from bad auth data - expect a functional, empty token.
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (4): ", err)
	}
	if err := pkcs11test.ChangeP11TestTokenAuthData(ctx, r, scratchpadPath, "auth3", "auth5"); err != nil {
		s.Error("Failed to change token using chaps_client (4): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if err != nil {
		s.Error("Failed to load token using chaps_client (5): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (1): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--replay_wifi"); err != nil {
		s.Error("Load token failed (2): ", err)
	}

	// Test 6: Token should have been recreated with 'auth3'.
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (5): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if err != nil {
		s.Error("Failed to load token using chaps_client (6): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
		s.Error("Token not valid after recovery: ", err)
	}

	// Test 7: Since token was recovered, previous correct auth should be rejected.
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (6): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth5")
	if err != nil {
		s.Error("Failed to load token using chaps_client (7): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (2): ", err)
	}

	// Test 8: Token should be invalid after scratchpad is removed
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth3")
	if err != nil {
		s.Error("Failed to load token using chaps_client (8): ", err)
	}
	if err = pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Cleanup failed (1): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err == nil {
		s.Error("Bad authorization data allowed (3): ", err)
	}
}
