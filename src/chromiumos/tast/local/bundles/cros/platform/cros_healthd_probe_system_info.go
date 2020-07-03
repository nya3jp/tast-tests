// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"path/filepath"
	"regexp"

	"chromiumos/tast/local/bundles/cros/platform/csv"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeSystemInfo,
		Desc: "Check that we can probe cros_healthd for system info",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
	})
}

func CrosHealthdProbeSystemInfo(ctx context.Context, s *testing.State) {
	const (
		// Location of cached VPD R/O contents
		cachedVpdRoPath = "/sys/firmware/vpd/ro/"
		// Location of cached VPD R/W contents
		cachedVpdRwPath = "/sys/firmware/vpd/rw/"
		// CrosConfig cros_healthd cached VPD path
		crosHealthdCachedVpdPath = "/cros-healthd/cached-vpd"
		// CrosConfig SKU number property
		skuNumberProperty = "has-sku-number"
		// Cached VPD Filenames
		firstPowerDateFileName  = "ActivateDate"
		manufactureDateFileName = "mfg_date"
		skuNumberFileName       = "sku_number"
		// Regex for first power date
		firstPowerDateRegex = "[0-9]{4}-[0-9]{2}"
		// Regex for manufacture date
		manufactureDateRegex = "[0-9]{4}-[0-9]{2}-[0-9]{2}"

		// CrosConfig ARC build properties path
		arcBuildPropertiesPath = "/arc/build-properties"
		// CrosConfig marketing name property
		marketingNameProperty = "marketing-name"

		// Location of DMI contents
		dmiPath = "/sys/class/dmi/id/"
		// DMI Filenames
		biosVersionFileName  = "bios_version"
		boardNameFileName    = "board_name"
		boardVersionFileName = "board_version"
		chassisTypeFileName  = "chassis_type"
		productNameFileName  = "product_name"
	)

	var ValidateCSV = csv.ValidateCSV
	var Dimensions = csv.Dimensions
	var Headers = csv.Headers
	var Column = csv.Column
	var UInt64 = csv.UInt64
	var TelemMatchRegex = csv.TelemMatchRegex
	var TelemEqualToFileContent = csv.TelemEqualToFileContent
	var TelemCheckFileContentIfFileShouldExist = csv.TelemCheckFileContentIfFileShouldExist
	var TelemEqualToCrosConfigContent = csv.TelemEqualToCrosConfigContent

	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategorySystem, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get system info: ", err)
	}

	err = ValidateCSV(records,
		Dimensions( /*expectedRows=*/ 2),
		Headers("first_power_date", "manufacture_date", "product_sku_number",
			"marketing_name", "bios_version", "board_name", "board_version",
			"chassis_type", "product_name"),
		Column(TelemEqualToFileContent(filepath.Join(cachedVpdRwPath, firstPowerDateFileName)),
			TelemMatchRegex(regexp.MustCompile(firstPowerDateRegex))),
		Column(TelemEqualToFileContent(filepath.Join(cachedVpdRoPath, manufactureDateFileName)),
			TelemMatchRegex(regexp.MustCompile(manufactureDateRegex))),
		Column(TelemCheckFileContentIfFileShouldExist(ctx, crosHealthdCachedVpdPath, skuNumberProperty,
			filepath.Join(cachedVpdRoPath, skuNumberFileName))),
		Column(TelemEqualToCrosConfigContent(ctx, arcBuildPropertiesPath, marketingNameProperty)),
		Column(TelemEqualToFileContent(filepath.Join(dmiPath, biosVersionFileName))),
		Column(TelemEqualToFileContent(filepath.Join(dmiPath, boardNameFileName))),
		Column(TelemEqualToFileContent(filepath.Join(dmiPath, boardVersionFileName))),
		Column(UInt64(), TelemEqualToFileContent(filepath.Join(dmiPath, chassisTypeFileName))),
		Column(TelemEqualToFileContent(filepath.Join(dmiPath, productNameFileName))))

	if err != nil {
		s.Error("Failed to validate CSV output: ", err)
	}
}
