// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/vendorutils"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HPCompliance,
		Desc: "Verifies that DUT satisfies all requirements to run Telemetry Extension such as has all required VPD fields and correct CrOSConfig",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:telemetry_extension_hw"},
		HardwareDeps: dep.HPModels(),
	})
}

// HPCompliance tests that DUT satisfies all requirements to run Telemetry Extension.
func HPCompliance(ctx context.Context, s *testing.State) {
	if vendor, err := vendorutils.FetchVendor(ctx); err != nil {
		s.Error("Failed to read vendor name: ", err)
	} else if got, want := vendor, "HP"; got != want {
		s.Errorf("Unexpected vendor name = got %q, want %q", got, want)
	}

	if oemDataBytes, err := testexec.CommandContext(ctx, "/usr/share/cros/oemdata.sh").Output(); err != nil {
		s.Error("Failed to get OEM data: ", err)
	} else if len(oemDataBytes) == 0 {
		s.Error("OEM data is empty")
	}

	if activateDateBytes, err := ioutil.ReadFile("/sys/firmware/vpd/rw/ActivateDate"); err != nil {
		s.Error("Failed to read ActivateDate VPD field: ", err)
	} else if len(activateDateBytes) == 0 {
		s.Error("ActivateDate VPD is empty")
	}

	if modelNameBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/model_name"); err != nil {
		s.Error("Failed to read model_name VPD field: ", err)
	} else if len(modelNameBytes) == 0 {
		s.Error("model_name VPD is empty")
	}

	if serialNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/serial_number"); err != nil {
		s.Error("Failed to read serial_number VPD field: ", err)
	} else if len(serialNumberBytes) == 0 {
		s.Error("serial_number VPD is empty")
	}

	if skuNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/sku_number"); err != nil {
		s.Error("Failed to read sku_number VPD field: ", err)
	} else if len(skuNumberBytes) == 0 {
		s.Error("sku_number VPD is empty")
	}

	if got, err := crosconfig.Get(ctx, "/cros-healthd/cached-vpd", "has-sku-number"); err != nil {
		s.Error("Failed to get has-sku-number value from cros config: ", err)
	} else if want := "true"; got != want {
		s.Errorf("Unexpected vendor name = got %q, want %q", got, want)
	}
}
