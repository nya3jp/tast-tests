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
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SanitizedUsernameAndSalt,
		Desc: "Verifies that sanitized username is the same across various ways to calculate it, and check that system salt is sane",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// getSanitizedUsernameAndCompare retrieves/computes the sanitized username of the given user with various methods (dbus and libbrillo), compare them to check that they match, and return the sanitized username if everything is alright.
func getSanitizedUsernameAndCompare(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, username string) (string, error) {
	fromBrillo, err := cryptohomeUtil.GetSanitizedUsername(ctx, username, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from libbrillo")
	}
	fromDBus, err := cryptohomeUtil.GetSanitizedUsername(ctx, username, true)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from dbus")
	}

	if fromBrillo != fromDBus {
		return "", errors.Errorf("sanitized username for %q differs between libbrillo (%q) and dbus (%q)", username, fromBrillo, fromDBus)
	}

	return fromBrillo, nil
}

// getSystemSaltAndCompare retrieves the system salt through various methods (dbus and libbrillo), compare them to check that they match, and return the hex encoded system salt if everything is alright.
func getSystemSaltAndCompare(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) (string, error) {
	fromBrillo, err := cryptohomeUtil.GetSystemSalt(ctx, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to get system salt from libbrillo")
	}
	fromDBus, err := cryptohomeUtil.GetSystemSalt(ctx, true)
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
		return "", errors.New("failed to decode system salt")
	}

	// Username should be lower case.
	username = strings.ToLower(username)

	// Compute the sanitized username with SHA1
	h := sha1.New()
	h.Write(binarySalt)
	h.Write([]byte(username))
	output := h.Sum(nil)

	sanitizedUsername := hex.EncodeToString(output)
	sanitizedUsername = strings.ToLower(sanitizedUsername)

	return sanitizedUsername, nil
}

func SanitizedUsernameAndSalt(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	cryptohomeUtil, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	// Check the sanitized username.
	firstSanitized, err := getSanitizedUsernameAndCompare(ctx, cryptohomeUtil, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to get sanitized username for first user: ", err)
	}

	secondSanitized, err := getSanitizedUsernameAndCompare(ctx, cryptohomeUtil, util.SecondUsername)
	if err != nil {
		s.Fatal("Failed to get sanitized username for second user: ", err)
	}

	if firstSanitized == secondSanitized {
		s.Fatal("Sanitized username is the same for two different users")
	}

	firstHomedir, err := cryptohome.UserPath(ctx, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to get home path for first user: ", err)
	}

	if !strings.Contains(firstHomedir, firstSanitized) {
		s.Fatal("Home path doesn't contain sanitized username")
	}

	// Check the system salt.
	systemSalt, err := getSystemSaltAndCompare(ctx, cryptohomeUtil)
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
