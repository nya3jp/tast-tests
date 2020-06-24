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

const (
	// Location of cached VPD RO contents
	cachedVpdRoPath = "/sys/firmware/vpd/ro/"
	// Location of cached VPD RW contents
	cachedVpdRwPath = "/sys/firmware/vpd/rw/"
	// Location of DMI contents
	dmiPath = "/sys/class/dmi/id/"
	// CrosConfig cros_healthd cached VPD path
	crosHealthdCachedVpdPath = "/cros-healthd/cached-vpd"
	// CrosConfig SKU number property
	skuNumberProperty = "has-sku-number"
	// CrosConfig ARC build properties path
	arcBuildPropertiesPath = "/arc/build-properties"
	// CrosConfig marketing name property
	marketingNameProperty = "marketing-name"
	// Cached VPD Filenames
	firstPowerDateFileName  = "ActivateDate"
	manufactureDateFileName = "mfg_date"
	skuNumberFileName       = "sku_number"
	// DMI Filenames
	biosVersionFileName  = "bios_version"
	boardNameFileName    = "board_name"
	boardVersionFileName = "board_version"
	chassisTypeFileName  = "chassis_type"
	productNameFileName  = "product_name"
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

func verifyCachedVpdInfo(ctx context.Context, s *testing.State, cachedVpdInfo []string) {
	// Check if the device has a first power date. If it does, the first
	// power date should be printed. If it does not, "NA" should be
	// printed.
	firstPowerDate, err := ioutil.ReadFile(filepath.Join(cachedVpdRwPath, firstPowerDateFileName))
	if os.IsNotExist(err) {
		if cachedVpdInfo[0] != "NA" {
			s.Fatalf("Incorrect first power date: got %v, want NA", cachedVpdInfo[0])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true first power date value: ", err)
	} else {
		if cachedVpdInfo[0] != string(firstPowerDate) {
			s.Fatalf("Failed to get correct first power date: got %v, want %v", cachedVpdInfo[0], string(firstPowerDate))
		}
	}
	// Check if the device has a manufacture date. If it does, the
	// manufacture date should be printed. If it does not, "NA" should be
	// printed.
	manufactureDate, err := ioutil.ReadFile(filepath.Join(cachedVpdRoPath, manufactureDateFileName))
	if os.IsNotExist(err) {
		if cachedVpdInfo[1] != "NA" {
			s.Fatalf("Incorrect manufacture date: got %v, want NA", cachedVpdInfo[1])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true manufacture date value: ", err)
	} else {
		if cachedVpdInfo[1] != string(manufactureDate) {
			s.Fatalf("Failed to get correct manufacture date: got %v, want %v", cachedVpdInfo[1], string(manufactureDate))
		}
	}
	// Check if the device has a sku number. If it does, the sku number should
	// be printed. If it does not, "NA" should be printed.
	val, err := crosconfig.Get(ctx, crosHealthdCachedVpdPath, skuNumberProperty)
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatalf("Failed to get %v property: %v", skuNumberProperty, err)
	}
	hasSku := err == nil && val == "true"
	if hasSku {
		skuNumber, err := ioutil.ReadFile(filepath.Join(cachedVpdRoPath, skuNumberFileName))
		if os.IsNotExist(err) {
			s.Fatal("Sku number was expected but is not present in: ", cachedVpdRoPath)
		} else if err != nil {
			s.Fatal("Failed to verify sku number: ", err)
		} else {
			if cachedVpdInfo[2] != string(skuNumber) {
				s.Fatalf("Failed to get correct sku number: got %v, want %v", cachedVpdInfo[2], string(skuNumber))
			}
		}
	} else {
		if cachedVpdInfo[2] != "NA" {
			s.Fatalf("Incorrect sku number: got %v, want NA", cachedVpdInfo[2])
		}
	}
}

func verifyCrosConfigInfo(ctx context.Context, s *testing.State, crosConfigInfo []string) {
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
	// Check if the device has a bios version. If it does, the
	// bios version should be printed. If it does not, "NA" should be
	// printed.
	content, err := ioutil.ReadFile(filepath.Join(dmiPath, biosVersionFileName))
	if os.IsNotExist(err) {
		if dmiInfo[0] != "NA" {
			s.Fatalf("Incorrect bios version: got %v, want NA", dmiInfo[0])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true bios version value: ", err)
	} else {
		biosVersion := strings.TrimRight(string(content), "\n")
		if dmiInfo[0] != biosVersion {
			s.Fatalf("Failed to get correct bios version: got %v, want %v", dmiInfo[0], biosVersion)
		}
	}
	// Check if the device has a board name. If it does, the
	// board name should be printed. If it does not, "NA" should be
	// printed.
	content, err = ioutil.ReadFile(filepath.Join(dmiPath, boardNameFileName))
	if os.IsNotExist(err) {
		if dmiInfo[1] != "NA" {
			s.Fatalf("Incorrect board name: got %v, want NA", dmiInfo[1])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true board name value: ", err)
	} else {
		boardName := strings.TrimRight(string(content), "\n")
		if dmiInfo[1] != boardName {
			s.Fatalf("Failed to get correct board name: got %v, want %v", dmiInfo[1], boardName)
		}
	}
	// Check if the device has a board version. If it does, the
	// board version should be printed. If it does not, "NA" should be
	// printed.
	content, err = ioutil.ReadFile(filepath.Join(dmiPath, boardVersionFileName))
	if os.IsNotExist(err) {
		if dmiInfo[2] != "NA" {
			s.Fatalf("Incorrect board version: got %v, want NA", dmiInfo[2])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true board version value: ", err)
	} else {
		boardVersion := strings.TrimRight(string(content), "\n")
		if dmiInfo[2] != boardVersion {
			s.Fatalf("Failed to get correct board version: got %v, want %v", dmiInfo[2], string(boardVersion))
		}
	}
	// Check if the device has a chassis type. If it does, the
	// chassis type should be printed. If it does not, "NA" should be
	// printed.
	content, err = ioutil.ReadFile(filepath.Join(dmiPath, chassisTypeFileName))
	if os.IsNotExist(err) {
		if dmiInfo[3] != "NA" {
			s.Fatalf("Incorrect chassis type: got %v, want NA", dmiInfo[3])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true chassis type value: ", err)
	} else {
		chassisType := strings.TrimRight(string(content), "\n")
		if dmiInfo[3] != chassisType {
			s.Fatalf("Failed to get correct chassis type: got %v, want %v", dmiInfo[3], chassisType)
		}
	}
	// Check if the device has a product name. If it does, the
	// product name should be printed. If it does not, "NA" should be
	// printed.
	content, err = ioutil.ReadFile(filepath.Join(dmiPath, productNameFileName))
	if os.IsNotExist(err) {
		if dmiInfo[4] != "NA" {
			s.Fatalf("Incorrect product name: got %v, want NA", dmiInfo[4])
		}
	} else if err != nil {
		s.Fatal("Failed to verify true product name value: ", err)
	} else {
		productName := strings.TrimRight(string(content), "\n")
		if dmiInfo[4] != productName {
			s.Fatalf("Failed to get correct product name: got %v, want %v", dmiInfo[4], productName)
		}
	}
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
