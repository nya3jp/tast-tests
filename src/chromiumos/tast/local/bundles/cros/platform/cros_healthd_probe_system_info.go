// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/local/crosconfig"
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
	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategorySystem, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get system info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of output lines: got %d want 2", len(records))
	}

	// Verify the headers.
	want := []string{"first_power_date", "manufacture_date",
		"product_sku_number", "marketing_name", "bios_version", "board_name",
		"board_version", "chassis_type", "product_name"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v want %v", got, want)
	}

	// Verify information that was retrieved from cached VPD.
	verifyCachedVpdInfo(ctx, s, records[1][0:3])

	// Verify information that was retrieved from CrosConfig.
	verifyCrosConfigInfo(ctx, s, records[1][3:4])

	// Verify information that was retrieved from DMI.
	verifyDmiInfo(s, records[1][4:])
}

func verifyCachedVpdInfo(ctx context.Context, s *testing.State, cachedVpdInfo []string) {
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
	)
	// Check if the device has a first power date. If it does, the first
	// power date should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, cachedVpdRwPath, firstPowerDateFileName, cachedVpdInfo[0], "first power date")
	// Check if the device has a manufacture date. If it does, the
	// manufacture date should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, cachedVpdRoPath, manufactureDateFileName, cachedVpdInfo[1], "manufacture date")
	// Check if the device has a sku number. If it does, the sku number should
	// be printed. If it does not, "NA" should be printed.
	val, err := crosconfig.Get(ctx, crosHealthdCachedVpdPath, skuNumberProperty)
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatalf("Failed to get %v property: %v", skuNumberProperty, err)
	}
	hasSku := err == nil && val == "true"
	if hasSku {
		validateActualValue(s, cachedVpdRoPath, skuNumberFileName, cachedVpdInfo[2], "sku number")
	} else {
		if cachedVpdInfo[2] != "NA" {
			s.Fatalf("Incorrect sku number: got %v, want NA", cachedVpdInfo[2])
		}
	}
}

func verifyCrosConfigInfo(ctx context.Context, s *testing.State, crosConfigInfo []string) {
	const (
		// CrosConfig ARC build properties path
		arcBuildPropertiesPath = "/arc/build-properties"
		// CrosConfig marketing name property
		marketingNameProperty = "marketing-name"
	)
	// Check that the system_info reported by cros_healthd has the correct
	// marketing name.
	val, err := crosconfig.Get(ctx, arcBuildPropertiesPath, marketingNameProperty)
	if err != nil {
		s.Fatalf("Failed to get %v property: %v", marketingNameProperty, err)
	}
	if crosConfigInfo[0] != val {
		s.Fatalf("Incorrect marketing name: got %v, want %v", crosConfigInfo[0], val)
	}
}

func verifyDmiInfo(s *testing.State, dmiInfo []string) {
	const (
		// Location of DMI contents
		dmiPath = "/sys/class/dmi/id/"
		// DMI Filenames
		biosVersionFileName  = "bios_version"
		boardNameFileName    = "board_name"
		boardVersionFileName = "board_version"
		chassisTypeFileName  = "chassis_type"
		productNameFileName  = "product_name"
	)
	// Check if the device has a bios version. If it does, the
	// bios version should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, dmiPath, biosVersionFileName, dmiInfo[0], "bios version")
	// Check if the device has a board name. If it does, the
	// board name should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, dmiPath, boardNameFileName, dmiInfo[1], "board name")
	// Check if the device has a board version. If it does, the
	// board version should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, dmiPath, boardVersionFileName, dmiInfo[2], "board version")
	// Check if the device has a chassis type. If it does, the
	// chassis type should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, dmiPath, chassisTypeFileName, dmiInfo[3], "chassis type")
	// Check if the device has a product name. If it does, the
	// product name should be printed. If it does not, "NA" should be
	// printed.
	validateActualValue(s, dmiPath, productNameFileName, dmiInfo[4], "product name")
}

// validateActualValue determines whether |fileName|, located in |fileDir|
// exists. If it does, it compares the stored value with |actualValue|. If it
// does not, it ensures that |actualValue| equals "NA". Use |printStr| to
// report errors.
func validateActualValue(s *testing.State, fileDir, fileName, actualValue, printStr string) {
	expectedValueByteArr, err := ioutil.ReadFile(filepath.Join(fileDir, fileName))
	if os.IsNotExist(err) {
		if actualValue != "NA" {
			s.Fatalf("Incorrect %v: got %v, want NA", printStr, actualValue)
		}
	} else if err != nil {
		s.Fatalf("Failed to verify true %v value: %v", printStr, err)
	} else {
		expectedValue := strings.TrimRight(string(expectedValueByteArr), "\n")
		if actualValue != expectedValue {
			s.Fatalf("Failed to get correct %v: got %v, want %v", printStr, actualValue, expectedValue)
		}
	}
}
