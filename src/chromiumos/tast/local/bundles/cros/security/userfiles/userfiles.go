// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package userfiles contains shared logic for security.UserFiles* tests.
package userfiles

import (
	"context"
	"strings"

	chk "chromiumos/tast/local/bundles/cros/security/filecheck"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	homeDir   = "/home/chronos" // standard home dir
	maxErrors = 10              // max errors to report per base dir
)

// Check checks files belonging to the supplied logged-in Chrome user.
// This is a helper function called by security.UserFiles* tests.
// Errors are reported via s.
func Check(ctx context.Context, s *testing.State, user string) {
	userDir, err := cryptohome.UserPath(user)
	if err != nil {
		s.Fatalf("Failed to get cryptohome dir for user %v: %v", user, err)
	}

	isChronosUID := chk.UID(sysutil.ChronosUID)

	chronosAccessGID, err := sysutil.GetGID("chronos-access")
	if err != nil {
		s.Fatal("Failed to find gid: ", err)
	}
	isChronosAccessGID := chk.GID(chronosAccessGID)

	checkPath := func(root string, patterns []*chk.Pattern) {
		s.Log("Checking ", root)
		probs, numPaths, err := chk.Check(ctx, root, patterns)
		if err != nil {
			s.Errorf("Failed to scan %v: %v", root, err)
		} else {
			s.Logf("Examined %d path(s) under %s", numPaths, root)
		}

		numErrors := 0
		for p, msgs := range probs {
			if numErrors++; numErrors > maxErrors {
				s.Error("Too many errors; aborting")
				break
			}
			s.Errorf("%v: %v", p, strings.Join(msgs, ", "))
		}
	}

	checkPath(homeDir, []*chk.Pattern{
		chk.NewPattern(chk.Path("user"), chk.SkipChildren()),
		chk.NewPattern(chk.Path("crash"), chk.SkipChildren()),
		chk.NewPattern(chk.PathRegexp(`^u-`), chk.SkipChildren()),
		chk.NewPattern(chk.PathRegexp(`^Singleton`)),
		chk.NewPattern(chk.Root(), isChronosUID, chk.Mode(0755)),
		chk.NewPattern(chk.AllPaths(), isChronosUID, chk.NotMode(022)),
	})

	checkPath(userDir, []*chk.Pattern{
		chk.NewPattern(chk.Path("Downloads"), isChronosUID, isChronosAccessGID, chk.Mode(0710), chk.SkipChildren()),
		chk.NewPattern(chk.Root(), isChronosUID, isChronosAccessGID, chk.Mode(0710)),
	})

	// TODO(derat): Add additional vault checks from security_ProfilePermissions.
}
