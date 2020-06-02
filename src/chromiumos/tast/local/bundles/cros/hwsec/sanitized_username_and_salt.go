// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
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

// getSanitizedUsernameAndCompare retrieves/computes the sanitized username of the given user with various methods (dbus and libbrillo), compares them to check that they match, and returns the sanitized username if everything is alright.
func getSanitizedUsernameAndCompare(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, username string) (string, error) {
	fromBrillo, err := cryptohomeUtil.GetSanitizedUsername(ctx, username, false /* don't use dbus */)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from libbrillo")
	}
	fromDBus, err := cryptohomeUtil.GetSanitizedUsername(ctx, username, true /* use dbus*/)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sanitized username from dbus")
	}

	if fromBrillo != fromDBus {
		return "", errors.Errorf("sanitized username for %q differs between libbrillo (%q) and dbus (%q)", username, fromBrillo, fromDBus)
	}

	return fromBrillo, nil
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
		s.Fatal("Error with sanitized username for first user: ", err)
	}

	secondSanitized, err := getSanitizedUsernameAndCompare(ctx, cryptohomeUtil, util.SecondUsername)
	if err != nil {
		s.Fatal("Error with sanitized username for second user: ", err)
	}

	if firstSanitized == secondSanitized {
		s.Fatalf("Sanitized username %q is the same for two different users (%q and %q)", firstSanitized, util.FirstUsername, util.SecondUsername)
	}

	usernames := []string{util.FirstUsername, util.SecondUsername}
	sanitizeds := []string{firstSanitized, secondSanitized}
	for i, u := range usernames {
		homedir, err := cryptohome.UserPath(ctx, u)
		if err != nil {
			s.Fatalf("Failed to get home path for %q: %v", u, err)
		}
		if !strings.Contains(homedir, sanitizeds[i]) {
			s.Fatalf("Home path %q doesn't contain sanitized username %q", homedir, sanitizeds[i])
		}
	}
}
