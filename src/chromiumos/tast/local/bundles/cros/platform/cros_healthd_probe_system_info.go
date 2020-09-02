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
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
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
		csv.Column("first_power_date", csv.EqualToFileContentOrNA(firstPowerDatePath),
			csv.MatchRegexOrNA(firstPowerDateRegex)),
		csv.Column("manufacture_date", csv.EqualToFileContentOrNA(manufactureDatePath),
			csv.MatchRegexOrNA(manufactureDateRegex)),
		csv.Column("product_sku_number", csv.EqualToFileIfCrosConfigPropOrNA(ctx, crosHealthdCachedVpdPath,
			skuNumberProperty, skuNumberPath)),
		csv.Column("marketing_name", csv.EqualToCrosConfigProp(ctx, arcBuildPropertiesPath, marketingNameProperty)),
		csv.Column("bios_version", csv.EqualToFileContentOrNA(biosVersionPath)),
		csv.Column("board_name", csv.EqualToFileContentOrNA(boardNamePath)),
		csv.Column("board_version", csv.EqualToFileContentOrNA(boardVersionPath)),
		csv.Column("chassis_type", csv.UInt64(), csv.EqualToFileContentOrNA(chassisTypePath)),
		csv.Column("product_name", csv.EqualToFileContentOrNA(productNamePath)),
		csv.Column("os_version", csv.MatchValue(osVersion)),
		csv.Column("os_channel", csv.MatchValue(osReleaseChannel)),
	)

	if err != nil {
		s.Error("Failed to validate CSV output: ", err)
	}
}
