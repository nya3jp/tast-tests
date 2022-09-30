// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FeatureVPDFields,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that VPD fields are properly set on the DUT",
		Contacts: []string{
			"lamzin@google.com",    // Telemetry Extension author
			"bkersting@google.com", // Test and Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr: []string{"group:telemetry_extension_hw"},
		Params: []testing.Param{
			{
				Name: "activate_date",
				Val:  "/sys/firmware/vpd/rw/ActivateDate",
			},
			{
				Name: "model_name",
				Val:  "/sys/firmware/vpd/ro/model_name",
			},
			{
				Name: "serial_number",
				Val:  "/sys/firmware/vpd/ro/serial_number",
			},
			{
				Name: "sku_number",
				Val:  "/sys/firmware/vpd/ro/serial_number",
			},
		},
	})
}

func FeatureVPDFields(ctx context.Context, s *testing.State) {
	vpdPath, ok := s.Param().(string)
	if !ok {
		s.Fatal("Failed to convert params value into string: ", s.Param())
	}

	if _, err := os.Stat(vpdPath); errors.Is(err, os.ErrNotExist) {
		s.Fatalf("VPD field %q does not exists", vpdPath)
	}

	bytes, err := ioutil.ReadFile(vpdPath)
	if err != nil {
		s.Fatalf("Unexpected err reading the VPD field at %q: %s", vpdPath, err)
	}

	if len(bytes) == 0 {
		s.Error("Unexpected empty VPD field")
	}
}
