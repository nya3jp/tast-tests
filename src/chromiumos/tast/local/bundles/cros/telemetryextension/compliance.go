// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Compliance,
		Desc: "Verifies that DUT satisfies all requirements to run Telemetry Extension such as has all required VPD fields and correct CrOSConfig",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "target_models",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
	})
}

// Compliance tests that DUT satisfies all requirements to run Telemetry Extension.
func Compliance(ctx context.Context, s *testing.State) {
	if vendor, err := fetchVendor(ctx); err != nil {
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

func fetchVendor(ctx context.Context) (string, error) {
	if got, err := crosconfig.Get(ctx, "/branding", "oem-name"); err != nil && !crosconfig.IsNotFound(err) {
		return "", errors.Wrap(err, "failed to get OEM name from CrOSConfig")
	} else if err == nil {
		return got, nil
	}

	if got, err := os.ReadFile("/sys/firmware/vpd/ro/oem_name"); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrap(err, "failed to get OEM name from VPD field")
	} else if err == nil {
		return string(got), nil
	}

	vendorBytes, err := os.ReadFile("/sys/devices/virtual/dmi/id/sys_vendor")
	if err != nil {
		return "", errors.Wrap(err, "failed to read vendor name")
	}

	vendor := strings.TrimSpace(string(vendorBytes))
	return vendor, nil
}
