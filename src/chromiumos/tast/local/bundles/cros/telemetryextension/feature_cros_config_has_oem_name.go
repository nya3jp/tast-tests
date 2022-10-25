// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FeatureCrOSConfigHasOEMName,
		Desc: "Verifies that CrOSConfig has OEM name",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr: []string{"group:telemetry_extension_hw"},
	})
}

// FeatureCrOSConfigHasOEMName tests that CrOSConfig has OEM name.
func FeatureCrOSConfigHasOEMName(ctx context.Context, s *testing.State) {
	contains := func(list []string, want string) bool {
		for _, elem := range list {
			if elem == want {
				return true
			}
		}
		return false
	}

	if vendor, err := crosconfig.Get(ctx, "/branding", "oem-name"); err != nil {
		s.Error("Failed to read vendor name: ", err)
	} else if got, allowedVendors := vendor, []string{"HP", "ASUS"}; !contains(allowedVendors, got) {
		s.Errorf("Unexpected vendor name = got %q, want %q", got, allowedVendors)
	}
}
