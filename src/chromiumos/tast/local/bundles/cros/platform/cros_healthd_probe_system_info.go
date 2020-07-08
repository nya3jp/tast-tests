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

		// CrosConfig ARC build properties path
		arcBuildPropertiesPath = "/arc/build-properties"
		// CrosConfig marketing name property
		marketingNameProperty = "marketing-name"

		// Location of DMI contents
		dmiPath = "/sys/class/dmi/id/"
	)

	var firstPowerDatePath = filepath.Join(cachedVpdRwPath, "ActivateDate")
	var manufactureDatePath = filepath.Join(cachedVpdRoPath, "mfg_date")
	var skuNumberPath = filepath.Join(cachedVpdRoPath, "sku_number")
	var biosVersionPath = filepath.Join(dmiPath, "bios_version")
	var boardNamePath = filepath.Join(dmiPath, "board_name")
	var boardVersionPath = filepath.Join(dmiPath, "board_version")
	var chassisTypePath = filepath.Join(dmiPath, "chassis_type")
	var productNamePath = filepath.Join(dmiPath, "product_name")
	var firstPowerDateRegex = regexp.MustCompile("[0-9]{4}-[0-9]{2}")
	var manufactureDateRegex = regexp.MustCompile("[0-9]{4}-[0-9]{2}-[0-9]{2}")

	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategorySystem, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get system info: ", err)
	}

	err = csv.ValidateCSV(records,
		csv.Rows(2),
		csv.Headers("first_power_date", "manufacture_date", "product_sku_number",
			"marketing_name", "bios_version", "board_name", "board_version",
			"chassis_type", "product_name"),
		csv.Column(csv.EqualToFileContentOrNA(firstPowerDatePath), csv.MatchRegexOrNA(firstPowerDateRegex)),
		csv.Column(csv.EqualToFileContentOrNA(manufactureDatePath), csv.MatchRegexOrNA(manufactureDateRegex)),
		csv.Column(csv.EqualToFileIfCrosConfigPropOrNA(ctx, crosHealthdCachedVpdPath,
			skuNumberProperty, skuNumberPath)),
		csv.Column(csv.EqualToCrosConfigProp(ctx, arcBuildPropertiesPath, marketingNameProperty)),
		csv.Column(csv.EqualToFileContentOrNA(biosVersionPath)),
		csv.Column(csv.EqualToFileContentOrNA(boardNamePath)),
		csv.Column(csv.EqualToFileContentOrNA(boardVersionPath)),
		csv.Column(csv.UInt64(), csv.EqualToFileContentOrNA(chassisTypePath)),
		csv.Column(csv.EqualToFileContentOrNA(productNamePath)))

	if err != nil {
		s.Error("Failed to validate CSV output: ", err)
	}
}
