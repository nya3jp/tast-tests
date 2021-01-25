// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/csv"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeSystemInfo,
		Desc: "Check that we can probe cros_healthd for system info",
		Contacts: []string{
			"cros-tdm@google.com",
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeSystemInfo(ctx context.Context, s *testing.State) {
	const (
		// Location of cached VPD R/O contents.
		cachedVpdRoPath = "/sys/firmware/vpd/ro/"
		// Location of cached VPD R/W contents.
		cachedVpdRwPath = "/sys/firmware/vpd/rw/"
		// CrosConfig cros_healthd cached VPD path.
		crosHealthdCachedVpdPath = "/cros-healthd/cached-vpd"
		// CrosConfig SKU number property.
		skuNumberProperty = "has-sku-number"

		// CrosConfig ARC build properties path.
		arcBuildPropertiesPath = "/arc/build-properties"
		// CrosConfig marketing name property.
		marketingNameProperty = "marketing-name"

		// Location of DMI contents.
		dmiPath = "/sys/class/dmi/id/"
	)

	var (
		firstPowerDatePath  = filepath.Join(cachedVpdRwPath, "ActivateDate")
		manufactureDatePath = filepath.Join(cachedVpdRoPath, "mfg_date")
		skuNumberPath       = filepath.Join(cachedVpdRoPath, "sku_number")
		serialNumberPath    = filepath.Join(cachedVpdRoPath, "serial_number")

		biosVersionPath  = filepath.Join(dmiPath, "bios_version")
		boardNamePath    = filepath.Join(dmiPath, "board_name")
		boardVersionPath = filepath.Join(dmiPath, "board_version")
		chassisTypePath  = filepath.Join(dmiPath, "chassis_type")
		productNamePath  = filepath.Join(dmiPath, "product_name")

		firstPowerDateRegex  = regexp.MustCompile("[0-9]{4}-[0-9]{2}")
		manufactureDateRegex = regexp.MustCompile("[0-9]{4}-[0-9]{2}-[0-9]{2}")
	)

	// Sanitize the marketing name value to remove commas. This matches the
	// behavior of the telem tool. For example
	// "Acer Chromebook Spin 11 (CP311-H1, CP311-1HN)" ->
	// "Acer Chromebook Spin 11 (CP311-H1/CP311-1HN)"
	// TODO(crbug/1135261): Remove these explicit values checks from the test
	marketingNameRaw, err := crosconfig.Get(ctx, arcBuildPropertiesPath, marketingNameProperty)
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Unable to get marketing name from cros_config: ", err)
	}
	marketingName := strings.ReplaceAll(marketingNameRaw, ", ", "/")

	lsbValues, err := lsbrelease.Load()
	if err != nil {
		s.Fatal("Failed to get lsb-release info: ", err)
	}
	versionComponents := []string{
		lsbValues[lsbrelease.Milestone],
		lsbValues[lsbrelease.BuildNumber],
		lsbValues[lsbrelease.PatchNumber],
	}
	osVersion := strings.Join(versionComponents, ".")
	osReleaseChannel := lsbValues[lsbrelease.ReleaseTrack]

	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get system info: ", err)
	}

	err = csv.ValidateCSV(records,
		csv.Rows(2),
		csv.ColumnWithDefault("first_power_date", croshealthd.NotApplicable, csv.EqualToFileContent(firstPowerDatePath),
			csv.MatchRegex(firstPowerDateRegex)),
		csv.ColumnWithDefault("manufacture_date", croshealthd.NotApplicable, csv.EqualToFileContent(manufactureDatePath),
			csv.MatchRegex(manufactureDateRegex)),
		csv.ColumnWithDefault("product_sku_number", croshealthd.NotApplicable, csv.EqualToFileIfCrosConfigProp(ctx, crosHealthdCachedVpdPath,
			skuNumberProperty, skuNumberPath)),
		csv.ColumnWithDefault("product_serial_number", croshealthd.NotApplicable, csv.EqualToFileContent(serialNumberPath)),
		csv.ColumnWithDefault("marketing_name", croshealthd.NotApplicable, csv.MatchValue(marketingName)),
		csv.ColumnWithDefault("bios_version", croshealthd.NotApplicable, csv.EqualToFileContent(biosVersionPath)),
		csv.ColumnWithDefault("board_name", croshealthd.NotApplicable, csv.EqualToFileContent(boardNamePath)),
		csv.ColumnWithDefault("board_version", croshealthd.NotApplicable, csv.EqualToFileContent(boardVersionPath)),
		csv.ColumnWithDefault("chassis_type", croshealthd.NotApplicable, csv.EqualToFileContent(chassisTypePath)),
		csv.ColumnWithDefault("product_name", croshealthd.NotApplicable, csv.EqualToFileContent(productNamePath)),
		csv.Column("os_version", csv.MatchValue(osVersion)),
		csv.Column("os_channel", csv.MatchValue(osReleaseChannel)),
	)

	if err != nil {
		s.Error("Failed to validate CSV output: ", err)
	}
}
