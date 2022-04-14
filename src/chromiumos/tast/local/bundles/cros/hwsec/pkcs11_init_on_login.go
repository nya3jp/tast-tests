// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Pkcs11InitOnLogin,
		Desc: "Tests if initialization of a user PKCS #11 token succeeds during login and if objects stored in the token persist through to a subsequent login",
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 4 * time.Minute,
	})
}

// Pkcs11InitOnLogin test the PKCS#11 behavior of initialization on login.
func Pkcs11InitOnLogin(ctx context.Context, s *testing.State) {
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

	// Create a user vault for the slot/token.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	// Measure pkcs11 init time
	timeStart := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := r.Run(ctx, "cryptohome", "--action=pkcs11_is_user_token_ok")
		return err
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		s.Fatal("Failed to init PKCS11: ", err)
	}
	timeElapsed := time.Since(timeStart)

	cleanupCtx := ctx
	// Give cleanup function 20 seconds to remove scratchpad.
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// Remove the user account after the test.
		if err := util.CleanupUserMount(ctx, cryptohome); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	sanitizedUsername, err := cryptohome.GetSanitizedUsername(ctx, util.FirstUsername, true)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}
	userChapsPath := "/run/daemon-store/chaps/" + sanitizedUsername
	lines, err := r.RunWithCombinedOutput(ctx, "chaps_client", "--list")
	if err != nil {
		s.Fatal("Failed to list token path: ", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(lines))
	// There should be only 1 system and 1 user slot
	slotCount := 0
	re := regexp.MustCompile(`Slot (\d+): `)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			slotCount++
		}
	}
	if slotCount != 2 {
		s.Error("Slot count is incorrect")
	}
	if !strings.Contains(string(lines), "Slot 1: "+userChapsPath) {
		s.Error("User slot number is incorrect")
	}

	// Test the token is properly initialized, including token name and token ownership
	if lines, err = r.RunWithCombinedOutput(ctx, "p11_replay", "--list_tokens"); err != nil {
		s.Error("Execute p11_replay inject and replay failed: ", err)
	}
	if !strings.Contains(string(lines), "Slot 1: "+"User TPM Token "+sanitizedUsername[:16]) {
		s.Error("User token name is incorrect")
	}

	const root = "/run/daemon-store/chaps"
	if _, err := os.Stat(filepath.Join(root, sanitizedUsername)); err != nil {
		s.Error("Chaps user directory doesn't exist: ", err)
	}
	if _, err := os.Stat(filepath.Join(root, sanitizedUsername, "database")); err != nil {
		s.Error("Chaps user database directory doesn't exist: ", err)
	}

	// Inject a key and make sure it's valid
	if lines, err = r.RunWithCombinedOutput(ctx, "p11_replay", "--slot=1", "--replay_wifi", "--inject"); err != nil {
		s.Error("Execute p11_replay inject and replay failed: ", err)
	}
	if !strings.Contains(string(lines), "Sign: CKR_OK") {
		s.Error("The PKCS #11 token is not available")
	}

	// Login again with the same account, make sure the token is still available and valid
	if _, err := cryptohome.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to unmount the user: ", err)
	}
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to re-mount the user: ", err)
	}
	if lines, err = r.RunWithCombinedOutput(ctx, "p11_replay", "--slot=1", "--replay_wifi", "--cleanup"); err != nil {
		s.Error("Execute p11_replay replay and cleanup failed: ", err)
	}
	if !strings.Contains(string(lines), "Sign: CKR_OK") {
		s.Error("The PKCS #11 token is not available after re-login")
	}

	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      "pkcs11_init",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, float64(timeElapsed.Milliseconds())/float64(util.ImportHWTimes))
	value.Save(s.OutDir())
}
