// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"time"

	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

const (
	preUpdateTimeoutN2N  = 1 * time.Minute
	postUpdateTimeoutN2N = 2 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicNToN,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test for the N2N update using Nebraska and test images",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: preUpdateTimeoutN2N + updateutil.UpdateTimeout + postUpdateTimeoutN2N,
	})
}

func BasicNToN(ctx context.Context, s *testing.State) {
	// Limit the timeout for the preparation steps.
	preCtx, cancel := context.WithTimeout(ctx, preUpdateTimeoutN2N)
	defer cancel()

	lsbContent := map[string]string{
		lsbrelease.Version:     "",
		lsbrelease.BuilderPath: "",
	}

	err := updateutil.FillFromLSBRelease(preCtx, s.DUT(), s.RPCHint(), lsbContent)
	if err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	// Original image version to compare it with the version after the update.
	originalVersion := lsbContent[lsbrelease.Version]
	// Builder path is used in selecting the update image.
	builderPath := lsbContent[lsbrelease.BuilderPath]

	// Update the DUT.
	if err := updateutil.UpdateFromGS(ctx, s.DUT(), s.OutDir(), s.RPCHint(), builderPath); err != nil {
		s.Fatalf("Failed to update DUT to image for %q from GS: %v", builderPath, err)
	}

	// Limit the timeout for the verification steps.
	postCtx, cancel := context.WithTimeout(ctx, postUpdateTimeoutN2N)
	defer cancel()

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the update")
	if err := s.DUT().Reboot(postCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after update: ", err)
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(postCtx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != originalVersion {
		s.Errorf("Image version changed after the update; got %s, want %s", version, originalVersion)
	}
}
