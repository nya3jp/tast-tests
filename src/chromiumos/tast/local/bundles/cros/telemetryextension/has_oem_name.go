// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/vendorutils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HasOEMName,
		Desc: "Verifies that DUT has correct OEM name",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr: []string{"group:telemetry_extension_hw"},
		Params: []testing.Param{
			{
				Name:              "asus",
				Val:               "ASUS",
				ExtraHardwareDeps: dep.AsusModels(),
			},
			{
				Name:              "hp",
				Val:               "HP",
				ExtraHardwareDeps: dep.HPModels(),
			},
		},
	})
}

// HasOEMName tests that DUT has correct OEM name which comes from
//   - /sys/devices/virtual/dmi/id/sys_vendor (old approach) or
//   - /sys/firmware/vpd/ro/oem_name (new approach for unreleased models) or
//   - CrOSConfig (new approach).
func HasOEMName(ctx context.Context, s *testing.State) {
	oemName, ok := s.Param().(string)
	if !ok {
		s.Fatal("Failed to convert params value into string: ", s.Param())
	}

	if vendor, err := vendorutils.FetchVendor(ctx); err != nil {
		s.Error("Failed to read vendor name: ", err)
	} else if got, want := vendor, oemName; got != want {
		s.Errorf("Unexpected vendor name: got %q, want %q", got, want)
	}
}
