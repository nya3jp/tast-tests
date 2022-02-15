// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
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

	helper, err := libhwseclocal.NewHelper(r)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	// Ensure that the user directory is unmounted and does not exist.
	if err := util.CleanupUserMount(ctx, cryptohome); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	scratchpadPath := "/run/daemon-store/chaps/"

	// Create a user vault for the slot/token.
	// Mount the vault of the user, so that we can test user keys as well.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	defer func() {
		// Remove the user account after the test.
		if err := util.CleanupUserMount(ctx, cryptohome); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	// Get the slot ID for the user vault.
	slot, err := cryptohome.GetTokenForUser(ctx, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to get user token slot ID: ", err)
	}

	sanitizedUsername, err := cryptohome.GetSanitizedUsername(ctx, util.FirstUsername, true)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}

	scratchpadPath = scratchpadPath + sanitizedUsername

	// Prepare the scratchpad.
	if err := pkcs11test.SetupP11TestToken(ctx, r, scratchpadPath, true); err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs at the end.
	defer pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath)

	// Give the cleanup 10 seconds to finish.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth1")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--inject", "--replay_wifi"); err != nil {
		s.Error("Failed to load test token (1): ", err)
	}
	// Change auth data while the token is not loaded.
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(shortenedCtx, r, scratchpadPath, "auth1", "auth2")
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth2")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (1): ", err)
	}
	// Change auth data while the token is loaded.
	pkcs11test.ChangeP11TestTokenAuthData(shortenedCtx, r, scratchpadPath, "auth2", "auth3")
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth3")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (2): ", err)
	}
	// Attempt change with incorrect current auth data.
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(shortenedCtx, r, scratchpadPath, "bad_auth", "auth4")
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth3")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi"); err != nil {
		s.Error("Change authorization data failed (3): ", err)
	}
	// Verify old auth data no longer works after change. This also verifies
	// recovery from bad auth data - expect a functional, empty token.
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	pkcs11test.ChangeP11TestTokenAuthData(shortenedCtx, r, scratchpadPath, "auth3", "auth5")
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth3")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (1): ", err)
	}
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--inject", "--replay_wifi"); err != nil {
		s.Error("Failed to load test token (2): ", err)
	}
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	// Token should have been recreated with 'auth3'.
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth3")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi"); err != nil {
		s.Error("Token not valid after recovery: ", err)
	}
	pkcs11test.UnloadP11TestToken(shortenedCtx, r, scratchpadPath)
	// Since token was recovered, previous correct auth should be rejected.
	pkcs11test.LoadP11TestToken(shortenedCtx, r, scratchpadPath, "auth5")
	if _, err := r.Run(shortenedCtx, "p11_replay", "--slot="+strconv.Itoa(slot), "--replay_wifi", "--skip_generate"); err == nil {
		s.Error("Bad authorization data allowed (2): ", err)
	}
}
