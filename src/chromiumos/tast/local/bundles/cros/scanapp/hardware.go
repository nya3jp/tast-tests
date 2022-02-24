// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Hardware,
		Desc: "Tests that the Scan app can be used on real hardware",
		Contacts: []string{
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr: []string{
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			// Format for test cases is as follows:
			// Name: manufacturer_model
			// Val: scanner descriptor file
			// ExtraData: scanner descriptor file
			{
				Name:      "canon_mf741c",
				Val:       "canon_mf741c_descriptor.json",
				ExtraData: []string{"canon_mf741c_descriptor.json"},
			}, {
				//MFP in lab
				Name:      "brother_dcp_l2550dw",
				Val:       "brother_dcp_l2550dw_descriptor.json",
				ExtraData: []string{"brother_dcp_l2550dw_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in lab
				Name:      "hp_laserjet_pro_m478",
				Val:       "hp_laserjet_pro_m478_descriptor.json",
				ExtraData: []string{"hp_laserjet_pro_m478_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in lab
				Name:      "sharp_mx_b467f",
				Val:       "sharp_mx_b467f_descriptor.json",
				ExtraData: []string{"sharp_mx_b467f_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			},
		},
	})
}

func Hardware(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	fileContents, err := ioutil.ReadFile(s.DataPath(s.Param().(string)))
	if err != nil {
		s.Fatal("Unable to read scanner descriptor file: ", err)
	}

	var scanner scanning.ScannerDescriptor
	err = json.Unmarshal(fileContents, &scanner)
	if err != nil {
		s.Fatal("Unable to unmarshal scanner descriptor file: ", err)
	}

	scanning.RunHardwareTests(ctx, s, cr, scanner)
}
