// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemHelperManifestVerification,
		Desc:         "Verifies the validity of the helper manifest",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		SoftwareDeps: []string{"modemfwd"},
	})
}

// ModemHelperManifestVerification Test
func ModemHelperManifestVerification(ctx context.Context, s *testing.State) {
	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}

	if !fileExists(cellular.GetModemFirmwareManifestPath()) {
		s.Fatal("Cannot find ", cellular.GetModemFirmwareManifestPath())
	}

	if !fileExists(cellular.GetModemHelperManifestPath()) {
		s.Fatal("Cannot find ", cellular.GetModemHelperManifestPath())
	}

	fwManifest, err := cellular.ParseModemFirmwareManifest(ctx, s)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	helperManifest, err := cellular.ParseModemHelperManifest(ctx, s)
	if err != nil {
		s.Fatal("Failed to parse the helper manifest: ", err)
	}

	modemHelperPath := cellular.GetModemHelperPath()
	variants := make(map[string]bool)
	for _, helper := range helperManifest.Helper {
		// Verify that we don't have multiple helpers per variant.
		for _, variant := range helper.Variant {
			if variants[variant] {
				s.Fatalf("The variant %q is present in multiple helpers", variant)
			}
			variants[variant] = true
		}

		// Verify that the helper exists.
		if helperPath := filepath.Join(modemHelperPath, helper.Filename); !fileExists(helperPath) {
			s.Fatal("Modem helper missing: ", helperPath)
		}
	}

	for _, device := range fwManifest.Device {
		// Verify that each variant has a helper.
		// If the helpers don't specify a list of variants, the helper is used for all variants.
		if len(variants) > 0 && !variants[device.Variant] {
			s.Fatalf("The variant %q has no modem helper. The variant is missing in %q", device.Variant, modemHelperPath)
		}
	}
}
