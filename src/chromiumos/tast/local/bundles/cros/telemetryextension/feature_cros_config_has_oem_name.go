// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
			"lamzin@google.com",    // Test and Telemetry Extension author
			"mgawad@google.com",    // Telemetry Extension author
			"bkersting@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr: []string{"group:telemetry_extension_hw"},
	})
}

// FeatureCrOSConfigHasOEMName tests that CrOSConfig has OEM name.
func FeatureCrOSConfigHasOEMName(ctx context.Context, s *testing.State) {
	if vendor, err := crosconfig.Get(ctx, "/branding", "oem-name"); err != nil {
		s.Error("Failed to read vendor name: ", err)
	} else if got, allowed_vendors := vendor, []string{"HP", "ASUS"}; !contains(allowed_vendors, got) {
		s.Errorf("Unexpected vendor name = got %q, want %q", got, allowed_vendors)
	}
}
