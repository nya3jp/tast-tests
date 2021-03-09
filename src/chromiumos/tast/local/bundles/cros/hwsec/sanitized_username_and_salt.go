// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SanitizedUsernameAndSalt,
		Desc: "Verifies that sanitized username is the same across various ways to calculate it, and check that system salt is valid",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

// getSanitizedUsernameAndCompare retrieves/computes the sanitized username of the given user with various methods (dbus and libbrillo), compares them to check that they match, and returns the sanitized username if everything is alright.
func getSanitizedUsernameAndCompare(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username string) (string, error) {
	fromBrillo, err := cryptohome.GetSanitizedUsername(ctx, username, false /* don't use dbus */)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from libbrillo")
	}
	fromDBus, err := cryptohome.GetSanitizedUsername(ctx, username, true /* use dbus*/)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from dbus")
	}

	if fromBrillo != fromDBus {
		return "", errors.Errorf("sanitized username for %q differs between libbrillo (%q) and dbus (%q)", username, fromBrillo, fromDBus)
	}

	return fromBrillo, nil
}

// getSystemSaltAndCompare retrieves the system salt through various methods (dbus and libbrillo), compares them to check that they match, and returns the hex encoded system salt if everything is alright.
func getSystemSaltAndCompare(ctx context.Context, cryptohome *hwsec.CryptohomeClient) (string, error) {
	fromBrillo, err := cryptohome.GetSystemSalt(ctx, false /* don't use dbus */)
	if err != nil {
		return "", errors.Wrap(err, "failed to get system salt from libbrillo")
	}
	fromDBus, err := cryptohome.GetSystemSalt(ctx, true /* use dbus */)
	if err != nil {
		return "", errors.Wrap(err, "failed to get system salt from dbus")
	}

	if fromBrillo != fromDBus {
		return "", errors.Errorf("system salt differs between libbrillo (%q) and dbus (%q)", fromBrillo, fromDBus)
	}

	return fromBrillo, nil
}

// computeSanitizedUsername compute the sanitized username for username, with the hex encoded system salt hexSalt.
func computeSanitizedUsername(hexSalt, username string) (string, error) {
	binarySalt, err := hex.DecodeString(hexSalt)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode system salt %q", hexSalt)
	}

	// Username should be lower case.
	username = strings.ToLower(username)

	// Compute the sanitized username with SHA1.
	h := sha1.New()
	h.Write(binarySalt)
	h.Write([]byte(username))
	output := h.Sum(nil)

	// Note that EncodeToString produces lower case output.
	sanitizedUsername := hex.EncodeToString(output)

	return sanitizedUsername, nil
}

func SanitizedUsernameAndSalt(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	// Check the sanitized username.
	firstSanitized, err := getSanitizedUsernameAndCompare(ctx, cryptohome, util.FirstUsername)
	if err != nil {
		s.Fatal("Error with sanitized username for first user: ", err)
	}

	secondSanitized, err := getSanitizedUsernameAndCompare(ctx, cryptohome, util.SecondUsername)
	if err != nil {
		s.Fatal("Error with sanitized username for second user: ", err)
	}

	if firstSanitized == secondSanitized {
		s.Fatalf("Sanitized username %q is the same for two different users (%q and %q)", firstSanitized, util.FirstUsername, util.SecondUsername)
	}

	usernames := []string{util.FirstUsername, util.SecondUsername}
	sanitizeds := []string{firstSanitized, secondSanitized}
	for i, u := range usernames {
		homedir, err := cryptohome.GetHomeUserPath(ctx, u)
		if err != nil {
			s.Fatalf("Failed to get home path for %q: %v", u, err)
		}
		if !strings.Contains(homedir, sanitizeds[i]) {
			s.Fatalf("Home path %q doesn't contain sanitized username %q", homedir, sanitizeds[i])
		}
	}

	// Check the system salt.
	systemSalt, err := getSystemSaltAndCompare(ctx, cryptohome)
	if err != nil {
		s.Fatal("Failed to get system salt: ", err)
	}

	// Compute the sanitized username and see if they match.
	computedFirstSanitized, err := computeSanitizedUsername(systemSalt, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to compute sanitizedUsername for first user: ", err)
	}
	if computedFirstSanitized != firstSanitized {
		s.Fatalf("Sanitized username mismatch for %q, libbrillo/dbus got %q, local computation got %q", util.FirstUsername, firstSanitized, computedFirstSanitized)
	}
}
