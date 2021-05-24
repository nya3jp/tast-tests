// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func: ChapsStress,
		Desc: "Repeatedly load/unload slots, create/remove keys, sign to check that chaps works as intended. This is designed to uncover flaws in chaps key/session reloading mechanism",
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		// Note: This is not in mainline because it takes too long to run.
		SoftwareDeps: []string{"chrome", "tpm2"},
		Timeout:      20 * time.Minute,
	})
}

const (
	// userCount is the number of users we want to test simultaneously.
	userCount = 4
	// keysPerUser is the maximum number of keys we'll have per user.
	keysPerUser = 16

	// turnCount is the number of turns we'll go through in one test run.
	turnCount = 1024

	// maxSignTimes is the maximum number of times we'll sign with a key in sign key turn.
	maxSignTimes = 4

	// The following are relative probabilities of events happening/being selected in runOneTurn().
	// mountProb is the probability that we mount a user's vault.
	mountProb = 10.0
	// unmountProb is the probability that we unmount all user's vault.
	unmountProb = 1.5
	// createKeyProb is the probability that we create a key.
	createKeyProb = 200.0
	// removeKeyProb is the probability that we remove a key.
	removeKeyProb = 20.0
	// signKeyProb is the probability that we sign a key.
	signKeyProb = 500.0
)

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

	// nextKeyID is the ID for the next created key
	nextKeyID int
	// mountCount is the number of times we mounted a vault.
	mountCount int
	// unmountCount is the number of times we unmounted all vaults.
	unmountCount int
	// createKeyCount is the number of times we created a key.
	createKeyCount int
	// removeKeyCount is the number of times we removed a key.
	removeKeyCount int
	// signCount is the number of times we signed with a key.
	signCount int
}

// getNextKeyID generates the label and keyID for the key that we're going to create.
func getNextKeyID(state *chapsStressState) (label, keyID string) {
	keyID = fmt.Sprintf("4242%06X", state.nextKeyID)
	label = "Key" + keyID
	state.nextKeyID++
	return label, keyID
}

// doMountUserTurn randomly selects a user that's not mounted and mount the vault.
func doMountUserTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) (retErr error) {
	var viableUsers []int
	for i := 0; i < userCount; i++ {
		if !state.mounted[i] {
			viableUsers = append(viableUsers, i)
		}
	}

	if len(viableUsers) == 0 {
		// All users are mounted, so let's skip.
		return nil
	}

	// Otherwise, select a user to mount.
	u := viableUsers[state.rand.Intn(len(viableUsers))]
	username := state.usernames[u]
	password := state.passwords[u]

	// Mount the vault.
	if err := cryptohome.MountVault(ctx, username, password, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount vault in mount user turn")
	}

	// Wait for it to be ready.
	if err := cryptohome.WaitForUserToken(ctx, username); err != nil {
		return errors.Wrap(err, "failed to wait for user token in mount user turn")
	}

	state.mounted[u] = true

	testing.ContextLogf(ctx, "Mounted user %d", u)
	state.mountCount++
	return nil
}

// doUnmountUserTurn unmounts all users.
func doUnmountUserTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) (retErr error) {
	// Unmount all users.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount all users in unmount user turn")
	}

	// Clear out the mounted state.
	for i := 0; i < userCount; i++ {
		state.mounted[i] = false
	}

	testing.ContextLog(ctx, "Unmounted all users")
	state.unmountCount++
	return nil
}

// doCreateKeyTurn randomly create a key for a random user.
func doCreateKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps, scratchpadPath string) (retErr error) {
	// Select a key that's not created.
	var viableKeys []int
	for i := 0; i < userCount*keysPerUser; i++ {
		if state.keys[i] == nil && state.mounted[i/keysPerUser] {
			viableKeys = append(viableKeys, i)
		}
	}

	if len(viableKeys) == 0 {
		// No key to create, not creating keys
		return nil
	}

	// Select the key.
	k := viableKeys[state.rand.Intn(len(viableKeys))]
	username := state.usernames[k/keysPerUser]

	// Next determine the key type that we'll generate.
	t := state.rand.Float64()

	// Now generate it.
	if t < 0.45 {
		// Imported key.
		label, keyID := getNextKeyID(state)
		key, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, label, keyID, false, true)
		if err != nil {
			return errors.Wrap(err, "failed to create imported key in create key turn")
		}
		state.keys[k] = key
	} else if t < 0.90 {
		// Software backed key.
		label, keyID := getNextKeyID(state)
		key, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, label, keyID, true, true)
		if err != nil {
			return errors.Wrap(err, "failed to create software-backed key in create key turn")
		}
		state.keys[k] = key
	} else {
		// Key generated in TPM.
		// This only happens 10% of the time because this is expensive in terms of run time.
		label, keyID := getNextKeyID(state)
		key, err := pkcs11Util.CreateGeneratedKey(ctx, scratchpadPath, pkcs11.GenRSA2048, username, label, keyID)
		if err != nil {
			return errors.Wrap(err, "failed to create software-backed key in create key turn")
		}
		state.keys[k] = key
	}

	state.createKeyCount++
	testing.ContextLogf(ctx, "Create key %d", k)
	return nil
}

// doRemoveKeyTurn removes a random key.
func doRemoveKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps) (retErr error) {
	// Select a key to remove
	var viableKeys []int
	for i := 0; i < userCount*keysPerUser; i++ {
		if state.keys[i] != nil && state.mounted[i/keysPerUser] {
			viableKeys = append(viableKeys, i)
		}
	}

	if len(viableKeys) == 0 {
		// No key to remove, not removing keys
		return nil
	}

	// Select the key.
	k := viableKeys[state.rand.Intn(len(viableKeys))]

	if err := pkcs11Util.DestroyKey(ctx, state.keys[k]); err != nil {
		return errors.Wrap(err, "failed to destroy key in remove key turn")
	}

	state.keys[k] = nil

	state.removeKeyCount++
	testing.ContextLogf(ctx, "Removed key %d", k)
	return nil
}

// doSignKeyTurn randomly select a key from a mounted vault to sign.
func doSignKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps, f1, f2 string) (retErr error) {
	// Select a key to sign
	var viableKeys []int
	for i := 0; i < userCount*keysPerUser; i++ {
		if state.keys[i] != nil && state.mounted[i/keysPerUser] {
			viableKeys = append(viableKeys, i)
		}
	}

	if len(viableKeys) == 0 {
		// No key to sign, not signing
		return nil
	}

	// Select the key and number of times to sign.
	k := viableKeys[state.rand.Intn(len(viableKeys))]
	times := state.rand.Intn(maxSignTimes)

	for i := 0; i < times; i++ {
		if err := pkcs11test.SignAndVerify(ctx, pkcs11Util, state.keys[k], f1, f2, &pkcs11.SHA256RSAPKCS); err != nil {
			return errors.Wrap(err, "failed to sign in sign key turn")
		}
	}

	testing.ContextLogf(ctx, "Signed with key %d", k)
	state.signCount++
	return nil
}

// runOneTurn runs one iteration of the test. It'll randomly choose to do one of the following:
// - Mount a user
// - Unmount all users
// - Create a key
// - Remove a key
// - Sign with a key
func runOneTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient, pkcs11Util *pkcs11.Chaps, scratchpadPath, f1, f2 string) (retErr error) {
	totalProb := mountProb + unmountProb + createKeyProb + removeKeyProb + signKeyProb
	accuProb := 0.0
	r := state.rand.Float64() * totalProb

	if r < accuProb+mountProb {
		return doMountUserTurn(ctx, state, cryptohome)
	}
	accuProb += mountProb

	if r < accuProb+unmountProb {
		return doUnmountUserTurn(ctx, state, cryptohome)
	}
	accuProb += unmountProb

	if r < accuProb+createKeyProb {
		return doCreateKeyTurn(ctx, state, pkcs11Util, scratchpadPath)
	}
	accuProb += createKeyProb

	if r < accuProb+removeKeyProb {
		return doRemoveKeyTurn(ctx, state, pkcs11Util)
	}
	accuProb += removeKeyProb

	return doSignKeyTurn(ctx, state, pkcs11Util, f1, f2)
}

func ChapsStress(ctx context.Context, s *testing.State) {
	r := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(r)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	pkcs11Util, err := pkcs11.NewChaps(ctx, r, cryptohome)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

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

	const scratchpadPath = "/tmp/ChapsECDSATest"

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
	defer func() {
		if err := cryptohome.UnmountAll(ctx); err != nil {
			s.Error("Failed to unmount after the test: ", err)
		}
		for _, user := range state.usernames {
			if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
				// Those vaults might not exist, so it's not fatal.
				s.Log("Failed to remove vault after the test: ", err)
			}
		}
	}()

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Give the cleanup 30 seconds to finish.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	for i := 0; i < turnCount; i++ {
		if err := runOneTurn(shortenedCtx, state, cryptohome, pkcs11Util, scratchpadPath, f1, f2); err != nil {
			s.Fatal("Turn failed: ", err)
		}
	}

	s.Logf("Mounted %d times, unmounted %d times, created %d keys, removed %d keys, signed %d times", state.mountCount, state.unmountCount, state.createKeyCount, state.removeKeyCount, state.signCount)
}
