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
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
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

		// mountCount is the times that we mount a user's vault.
		mountCount = 15
		// unmountCount is the times that we unmount all user's vault.
		unmountCount = 2
		// createSoftwareKeyCount is the times that we create a software-backed key.
		createSoftwareKeyCount = 110
		// createImportedKeyCount is the times that we create a key then import it.
		createImportedKeyCount = 110
		// createGeneratedKeyCount is the times that we generate a key in TPM.
		createGeneratedKeyCount = 20
		// removeKeyCount is the times that we remove a key.
		removeKeyCount = 25
		// signKeyCount is the times that we sign a key.
		signKeyCount = 650
	)

	r := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(r, s.DUT())
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
		state.passwords[i] = fmt.Sprintf("u%d.%s", i, util.FirstPassword1)
	}
	state.mounted = make([]bool, userCount)
	state.keys = make([]*pkcs11.KeyInfo, userCount*keysPerUser)
	// Seed the random with a deterministic seed for reproducible run.
	state.rand = rand.New(rand.NewSource(42))
	state.userCount = userCount
	state.keysPerUser = keysPerUser

	const scratchpadPath = "/tmp/ChapsECDSATest"

	// Give the cleanup 30 seconds to finish.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*30)
	defer cancel()

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	cleanupCryptohome := func(ctx context.Context) {
		if err := cryptohome.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount for cleanup: ", err)
		}
		for _, user := range state.usernames {
			if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
				// Those vaults might not exist, so it's not fatal.
				s.Log("Failed to remove vault for cleanup: ", err)
			}
		}
	}
	cleanupCryptohome(ctx)
	defer cleanupCryptohome(ctxForCleanup)

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctxForCleanup, r, scratchpadPath)

	// Create an array with all the possible rounds and shuffle them.
	rounds := make([]roundType, 0)
	rounds = append(rounds, fillRound(mountRound, mountCount)...)
	rounds = append(rounds, fillRound(unmountRound, unmountCount)...)
	rounds = append(rounds, fillRound(createSoftwareKeyRound, createSoftwareKeyCount)...)
	rounds = append(rounds, fillRound(createImportedKeyRound, createImportedKeyCount)...)
	rounds = append(rounds, fillRound(createGeneratedKeyRound, createGeneratedKeyCount)...)
	rounds = append(rounds, fillRound(removeKeyRound, removeKeyCount)...)
	rounds = append(rounds, fillRound(signKeyRound, signKeyCount)...)
	state.rand.Shuffle(len(rounds), func(x, y int) {
		rounds[x], rounds[y] = rounds[y], rounds[x]
	})

	for i := 0; i < len(rounds); i++ {
		if err := runOneTurn(ctx, state, cryptohome, rounds[i], pkcs11Util, scratchpadPath, f1, f2); err != nil {
			s.Fatal("Turn failed: ", err)
		}
	}

	s.Logf("Mounted %d times, unmounted %d times, created %d keys, removed %d keys, signed %d times", state.mountCount, state.unmountCount, state.createKeyCount, state.removeKeyCount, state.signCount)
}

// roundType is the type for determining what to do in a round, it's an enum.
type roundType int64

// Below are the possile round types (enums).
const (
	mountRound roundType = iota
	unmountRound
	createSoftwareKeyRound
	createImportedKeyRound
	createGeneratedKeyRound
	removeKeyRound
	signKeyRound
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
	// nextKeyID is the ID for the next created key
	nextKeyID int

	// Below are some statistics of the stress test run:

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

	// userCount is the number of users that we're testing simultaneously.
	userCount int
	// keysPerUser is the maximum number of keys we'll have per user.
	keysPerUser int
}

// getNextKeyID generates the label and keyID for the key that we're going to create.
func getNextKeyID(state *chapsStressState) (label, keyID string) {
	keyID = fmt.Sprintf("4242%06X", state.nextKeyID)
	label = "Key" + keyID
	state.nextKeyID++
	return label, keyID
}

// doMountUserTurn randomly selects a user that's not mounted and mount the vault.
func doMountUserTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) error {
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
	if err := cryptohome.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(username, password), true, hwsec.NewVaultConfig()); err != nil {
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
func unmountAll(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient) error {
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

// doCreateKeyTurn randomly create a key for a random user.
func doCreateKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps, scratchpadPath string, round roundType) error {
	// Select a key that's not created.
	var viableKeys []int
	for i := 0; i < state.userCount*state.keysPerUser; i++ {
		if state.keys[i] == nil && state.mounted[i/state.keysPerUser] {
			viableKeys = append(viableKeys, i)
		}
	}

	if len(viableKeys) == 0 {
		// No key to create, not creating keys
		return nil
	}

	// Select the key.
	k := viableKeys[state.rand.Intn(len(viableKeys))]
	username := state.usernames[k/state.keysPerUser]

	createKeyFunc := func(label, keyID string) (*pkcs11.KeyInfo, error) {
		return nil, errors.New("unused create key function called")
	}
	if round == createImportedKeyRound {
		createKeyFunc = func(label, keyID string) (*pkcs11.KeyInfo, error) {
			return pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, label, keyID, false, true)
		}
	} else if round == createSoftwareKeyRound {
		createKeyFunc = func(label, keyID string) (*pkcs11.KeyInfo, error) {
			return pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, label, keyID, true, true)
		}
	} else if round == createGeneratedKeyRound {
		createKeyFunc = func(label, keyID string) (*pkcs11.KeyInfo, error) {
			return pkcs11Util.CreateGeneratedKey(ctx, scratchpadPath, pkcs11.GenRSA2048, username, label, keyID)
		}
	} else {
		return errors.New("invalid round in doCreateKeyTurn")
	}

	label, keyID := getNextKeyID(state)
	key, err := createKeyFunc(label, keyID)
	if err != nil {
		return errors.Wrapf(err, "failed to create key for round %d", round)
	}
	state.keys[k] = key

	state.createKeyCount++
	testing.ContextLogf(ctx, "Create key %d", k)
	return nil
}

// doRemoveKeyTurn removes a random key.
func doRemoveKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps) error {
	// Select a key to remove
	var viableKeys []int
	for i := 0; i < state.userCount*state.keysPerUser; i++ {
		if state.keys[i] != nil && state.mounted[i/state.keysPerUser] {
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
func doSignKeyTurn(ctx context.Context, state *chapsStressState, pkcs11Util *pkcs11.Chaps, f1, f2 string) error {
	const (
		// maxSignTimes is the maximum number of times we'll sign with a key in sign key turn.
		maxSignTimes = 4
	)
	// Select a key to sign
	var viableKeys []int
	for i := 0; i < state.userCount*state.keysPerUser; i++ {
		if state.keys[i] != nil && state.mounted[i/state.keysPerUser] {
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

// runOneTurn runs one iteration of the test. It'll do one of the following according to round:
// - Mount a user
// - Unmount all users
// - Create a key
// - Remove a key
// - Sign with a key
func runOneTurn(ctx context.Context, state *chapsStressState, cryptohome *hwsec.CryptohomeClient, round roundType, pkcs11Util *pkcs11.Chaps, scratchpadPath, f1, f2 string) error {
	if round == mountRound {
		return doMountUserTurn(ctx, state, cryptohome)
	}

	if round == unmountRound {
		return unmountAll(ctx, state, cryptohome)
	}

	if round == createSoftwareKeyRound || round == createImportedKeyRound || round == createGeneratedKeyRound {
		return doCreateKeyTurn(ctx, state, pkcs11Util, scratchpadPath, round)
	}

	if round == removeKeyRound {
		return doRemoveKeyTurn(ctx, state, pkcs11Util)
	}

	if round == signKeyRound {
		return doSignKeyTurn(ctx, state, pkcs11Util, f1, f2)
	}

	return errors.New("invalid round in runOneTurn")
}
