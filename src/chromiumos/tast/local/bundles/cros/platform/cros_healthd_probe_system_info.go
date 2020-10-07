// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/local/bundles/cros/platform/csv"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeSystemInfo,
		Desc: "Check that we can probe cros_healthd for system info",
		Contacts: []string{
			"cros-tdm@google.com",
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("sand", "lava")), // TODO(crbug/1134667)
	})
}

func CrosHealthdProbeSystemInfo(ctx context.Context, s *testing.State) {
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
	if err != nil {
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

	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategorySystem, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get system info: ", err)
	}

	err = csv.ValidateCSV(records,
		csv.Rows(2),
		csv.Column("first_power_date", csv.EqualToFileContentOr(firstPowerDatePath, croshealthd.NotApplicableString),
			csv.MatchRegexOr(firstPowerDateRegex, croshealthd.NotApplicableString)),
		csv.Column("manufacture_date", csv.EqualToFileContentOr(manufactureDatePath, croshealthd.NotApplicableString),
			csv.MatchRegexOr(manufactureDateRegex, croshealthd.NotApplicableString)),
		csv.Column("product_sku_number", csv.EqualToFileIfCrosConfigPropOr(ctx, crosHealthdCachedVpdPath,
			skuNumberProperty, skuNumberPath, croshealthd.NotApplicableString)),
		csv.Column("marketing_name", csv.MatchValue(marketingName)),
		csv.Column("bios_version", csv.EqualToFileContentOr(biosVersionPath, croshealthd.NotApplicableString)),
		csv.Column("board_name", csv.EqualToFileContentOr(boardNamePath, croshealthd.NotApplicableString)),
		csv.Column("board_version", csv.EqualToFileContentOr(boardVersionPath, croshealthd.NotApplicableString)),
		csv.Column("chassis_type", csv.EqualToFileContentOr(chassisTypePath, croshealthd.NotApplicableString)),
		csv.Column("product_name", csv.EqualToFileContentOr(productNamePath, croshealthd.NotApplicableString)),
		csv.Column("os_version", csv.MatchValue(osVersion)),
		csv.Column("os_channel", csv.MatchValue(osReleaseChannel)),
	)

	if err != nil {
		s.Error("Failed to validate CSV output: ", err)
	}
}
