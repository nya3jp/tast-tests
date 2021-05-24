// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChapsStress,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Repeatedly load/unload slots, create/remove keys, sign to check that chaps works as intended. This is designed to uncover flaws in chaps key/session reloading mechanism",
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		// Note: This is not in mainline because it takes too long to run.
		SoftwareDeps: []string{"chrome", "tpm2"},
		Timeout:      20 * time.Minute,
	})
}

func ChapsStress(ctx context.Context, s *testing.State) {
	const (
		// userCount is the number of users we want to test simultaneously.
		userCount = 4
		// keysPerUser is the maximum number of keys we'll have per user.
		keysPerUser = 16

		// The following are counts of events happening in the test.

		// mountCount is the times that we mount a user's vault.
		mountCount = 15
		// unmountCount is the times that we unmount all user's vault.
		unmountCount = 2
	)

	r := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(r)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	// Prepare the states for this test and the user/pass lists.
	state := &chapsStressState{}
	state.usernames = make([]string, userCount)
	state.passwords = make([]string, userCount)
	for i := 0; i < userCount; i++ {
		state.usernames[i] = fmt.Sprintf("u%d.%s", i, util.FirstUsername)
		state.passwords[i] = fmt.Sprintf("u%d.%s", i, util.FirstPassword)
	}
	state.mounted = make([]bool, userCount)
	state.keys = make([]*pkcs11.KeyInfo, userCount*keysPerUser)
	// Seed the random with a deterministic seed for reproducible run.
	state.rand = rand.New(rand.NewSource(42))
	state.userCount = userCount

	const scratchpadPath = "/tmp/ChapsECDSATest"

	// Give the cleanup 30 seconds to finish.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount before the start of test: ", err)
	}
	for _, user := range state.usernames {
		if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
			// Those vaults probably don't exist, so it's not fatal.
			s.Log("Failed to remove vault before the start of test: ", err)
		}
	}
	defer func(ctx context.Context) {
		if err := cryptohome.UnmountAll(ctx); err != nil {
			s.Error("Failed to unmount after the test: ", err)
		}
		for _, user := range state.usernames {
			if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
				// Those vaults might not exist, so it's not fatal.
				s.Log("Failed to remove vault after the test: ", err)
			}
		}
	}(ctxForCleanup)

	// Prepare the scratchpad.
	_, _, err = pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctxForCleanup, r, scratchpadPath)

	// Create an array with all the possible rounds and shuffle them.
	rounds := make([]roundType, 0)
	rounds = append(rounds, fillRound(mountRound, mountCount)...)
	rounds = append(rounds, fillRound(unmountRound, unmountCount)...)
	state.rand.Shuffle(len(rounds), func(x, y int) {
		rounds[x], rounds[y] = rounds[y], rounds[x]
	})

	for i := 0; i < len(rounds); i++ {
		if err := runOneTurn(ctx, state, cryptohome, rounds[i]); err != nil {
			s.Fatal("Turn failed: ", err)
		}
	}

	s.Logf("Mounted %d times, unmounted %d times", state.mountCount, state.unmountCount)
}

// roundType is the type for determining what to do in a round, it's an enum.
type roundType int64

// Below are the possile round types (enums).
const (
	mountRound roundType = iota
	unmountRound
)

// fillRound is a simple helper that create an array of size count filled with round.
func fillRound(round roundType, count int) []roundType {
	rounds := make([]roundType, count)
	for i := 0; i < count; i++ {
		rounds[i] = round
	}
	return rounds
}

// chapsStressState is a struct that stores the state of the test (i.e. if a user is mounted or
// if a key is loaded) while the test is running.
type chapsStressState struct {
	// usernames is the list of usernames used in the test.
	usernames []string
	// passwords is the list of passwords used in the test. It corresponds to the
	// usernames above.
	passwords []string
	// mounted records whether a user is mounted. It corresponds to the usernames above.
	mounted []bool
	// keys are the keys for each user. [0:keysPerUser] is for the first user,
	// [keysPerUser:2*keysPerUser] is for the second user... etc.
	// If that key is not created/has been deleted, then it'll be nil.
	keys []*pkcs11.KeyInfo
	// rand is the source of randomness that is used throughout the test to ensure that
	// it is deterministic.
	rand *rand.Rand

	// Below are some statistics of the stress test run:

	// mountCount is the number of times we mounted a vault.
	mountCount int
	// unmountCount is the number of times we unmounted all vaults.
	unmountCount int

	// userCount is the number of users that we're testing simultaneously.
	userCount int
}

// doMountUserTurn randomly selects a user that's not mounted and mount the vault.
func doMountUserTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) (retErr error) {
	var viableUsers []int
	for i := 0; i < state.userCount; i++ {
		if !state.mounted[i] {
			viableUsers = append(viableUsers, i)
		}
	}

	if len(viableUsers) == 0 {
		// All users are mounted, so let's skip.
		testing.ContextLog(ctx, "Skipping mount round because all users are mounted")
		return nil
	}

	// Otherwise, select a user to mount.
	u := viableUsers[state.rand.Intn(len(viableUsers))]
	username := state.usernames[u]
	password := state.passwords[u]

	// Mount the vault.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(username, password), true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrapf(err, "failed to mount vault in mount user turn for user %d", u)
	}

	// Wait for it to be ready.
	if err := cryptohome.WaitForUserToken(ctx, username); err != nil {
		return errors.Wrapf(err, "failed to wait for user token in mount user turn for user %d", u)
	}

	state.mounted[u] = true

	testing.ContextLogf(ctx, "Mounted user %d", u)
	state.mountCount++
	return nil
}

// unmountAll unmounts all users.
func unmountAll(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) (retErr error) {
	// Unmount all users.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount all users in unmount user turn")
	}

	// Clear out the mounted state.
	for i := 0; i < state.userCount; i++ {
		state.mounted[i] = false
	}

	testing.ContextLog(ctx, "Unmounted all users")
	state.unmountCount++
	return nil
}

// runOneTurn runs one iteration of the test. It'll do one of the following according to round:
// - Mount a user
// - Unmount all users
func runOneTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient, round roundType) (retErr error) {
	if round == mountRound {
		return doMountUserTurn(ctx, state, cryptohome)
	}

	if round == unmountRound {
		return unmountAll(ctx, state, cryptohome)
	}

	panic("Invalid round in runOneTurn")
}
