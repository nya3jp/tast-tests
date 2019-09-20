// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WifiCheckSARTable,
		Desc: "Runs a preliminary check on device SAR tables for devices with Intel WiFi",
		Contacts: []string{
			"kglund@google.com",               // Author
			"chromeos-kernel-wifi@google.com", // WiFi team
		},
		// Attr "disabled" because this test should only be run manually,
		// not with the CQ or any test suite.
		Attr:         []string{"informational", "disabled"},
		SoftwareDeps: []string{"wifi"},
	})
}

// limitCompliance provides a type that expresses whether a set of
// values exceed allowable limits.
type limitCompliance int

const (
	outsideHardLimits limitCompliance = iota
	outsideSoftLimits
	withinLimits
)

// getWifiVendorID returns the vendor ID of the given wireless network interface, or returns an error on failure.
func getWifiVendorID(ctx context.Context, netIf string) (string, error) {
	devicePath := filepath.Join("/sys/class/net", netIf, "device")

	vendorID, err := ioutil.ReadFile(filepath.Join(devicePath, "vendor"))
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get vendor ID", netIf)
	}
	return strings.TrimSpace(string(vendorID)), nil
}

// getValuesFromASLWithKey parses ASL formatted data and returns
// the array of integers from the body of the section labeled with the given key.
// TODO(kglund) unit test this function
func getValuesFromASLWithKey(data []byte, key string) ([]int64, error) {
	dataString := string(data)
	// Remove spaces and newlines from from data to make parsing easier.
	dataString = strings.Replace(dataString, "\n", "", -1)
	dataString = strings.Replace(dataString, " ", "", -1)
	// Try to find the key within the data.
	keyIndex := strings.Index(dataString, "Name("+key+",Package")
	if keyIndex == -1 {
		return nil, errors.New("could not find key in data")
	}
	// Below is an example of the format for the ASL data being parsed.
	// Parse by first finding the key - in this case "WRDS" - and then moving
	// two open brackets down to reach the body of the data, and end on the
	// next closed bracket.
	//
	// Name (WRDS, Package (0x02)
	// {
	// 		0x00000000,
	// 		Package (0x0C)
	// 		{
	// 			0x00000007,
	// 			0x00000001,
	// 			0x80,
	// 			0x88,
	// 			0x84,
	// 			0x80,
	// 			0x88,
	// 			0x80,
	// 			0x88,
	// 			0x84,
	// 			0x80,
	// 			0x88
	// 		}
	// })
	startIndex := strings.Index(dataString[keyIndex:], "{") + keyIndex + 1
	startIndex = strings.Index(dataString[startIndex:], "{") + startIndex + 1
	endIndex := strings.Index(dataString[startIndex:], "}") + startIndex
	values := strings.Split(dataString[startIndex:endIndex], ",")
	var intValues []int64
	for _, val := range values {
		intVal, err := strconv.ParseInt(val, 0, 64)
		if err != nil {
			return nil, errors.New("invalid ASL format or key")
		}
		intValues = append(intValues, intVal)
	}
	return intValues, nil
}

// sarLimits structs represent a set of soft and hard limits for SAR values.
type sarLimits struct {
	HardMax float64
	SoftMax float64
	HardMin float64
	SoftMin float64
}

// parseAndVerifySARValues takes in a slice of int values and compares them
// against the provided SAR limits. It returns a limitCompliance struct
// which expresses the degree to which the values fit within the limits.
// SAR values are stored in their raw form as ints, which are decoded by this
// function into the floats they represent.
func parseAndVerifySARValues(SARValues []int64, limits sarLimits) ([]float64, limitCompliance) {
	var realSARValues []float64
	exceedsSoftLimits := false
	exceedsHardLimits := false
	for _, val := range SARValues {
		// Actual SAR values are 1/8 * the stored ints.
		realSARValue := float64(val) / 8
		realSARValues = append(realSARValues, realSARValue)
		if realSARValue < limits.SoftMin || realSARValue > limits.SoftMax {
			exceedsSoftLimits = true
		}
		if realSARValue < limits.HardMin || realSARValue > limits.HardMax {
			exceedsHardLimits = true
		}
	}
	if exceedsHardLimits {
		return realSARValues, outsideHardLimits
	}
	if exceedsSoftLimits {
		return realSARValues, outsideSoftLimits
	}
	return realSARValues, withinLimits
}

func WifiCheckSARTable(ctx context.Context, s *testing.State) {
	const (
		// These values represent the allowable SAR limits for clamshell and tablet modes.
		tabletHardMax    = 14.0
		tabletSoftMax    = 16.0
		tabletHardMin    = 6.0
		tabletSoftMin    = 8.0
		clamshellSoftMax = 20.0
		clamshellHardMax = 22.0
		clamshellSoftMin = 12.0
		clamshellHardMin = 10.0
		// SSDT (Secondary System Description Table) contains SAR data
		// in encoded binary format.
		pathToSSDT = "/sys/firmware/acpi/tables/SSDT"
		// Vendor ID for Intel WiFi.
		intelVendorID = "0x8086"
	)

	// Verify that the DUT uses Intel WiFi.
	netIf, err := shill.GetWifiInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get network interface name: ", err)
	}
	deviceVendorID, err := getWifiVendorID(ctx, netIf)
	if err != nil {
		s.Fatal("Failed to get device name: ", err)
	}
	if deviceVendorID != intelVendorID {
		s.Fatal("Wrong Vendor: This test only runs on devices which use Intel WiFi")
	}

	SSDTRaw, err := ioutil.ReadFile(pathToSSDT)
	if err != nil {
		s.Fatal("Could not read SSDT data: ", err)
	}
	// Write encoded SSDT data to temp file.
	tmpSSDT, err := ioutil.TempFile("", "tempSSDT")
	if err != nil {
		s.Fatal("Could not create temp file for SSDT: ", err)
	}
	defer os.Remove(tmpSSDT.Name())
	if _, err := tmpSSDT.Write(SSDTRaw); err != nil {
		s.Fatal("Could not write to temp SSDT file: ", err)
	}

	// Use iasl to decode the data into ASL format.
	cmd := testexec.CommandContext(ctx, "iasl", "-d", tmpSSDT.Name())
	if err = cmd.Run(); err != nil {
		s.Fatal("Could not run iasl on Dut: ", err)
	}
	// Read in the decoded table.
	pathToDecodedSSDT := tmpSSDT.Name() + ".dsl"
	decodedSSDT, err := ioutil.ReadFile(pathToDecodedSSDT)
	if err != nil {
		s.Fatal("SSDT decoding failed: ", err)
	}
	defer os.Remove(pathToDecodedSSDT)

	// WRDS stores tablet mode SAR tables.
	tabletSAR, err := getValuesFromASLWithKey(decodedSSDT, "WRDS")
	if err != nil {
		s.Error("Unable to find SAR values, does your device support SAR: ", err)
	}

	tabletLimits := sarLimits{
		HardMax: tabletHardMax,
		HardMin: tabletHardMin,
		SoftMax: tabletSoftMax,
		SoftMin: tabletSoftMin,
	}
	// Get and verify the real tablet SAR values.
	tabletRealSARValues, tabletCompliance := parseAndVerifySARValues(tabletSAR[2:], tabletLimits)
	s.Logf("Tablet SAR table: %.3f", tabletRealSARValues)
	switch tabletCompliance {
	case outsideHardLimits:
		s.Error("Tablet SAR values exceed limits, requires manual approval")
	case outsideSoftLimits:
		s.Log("WARNING: Tablet SAR values are near allowable limits")
	default:
		s.Log("Tablet SAR values are within allowable limits")
	}

	// EWRD stores clamshell mode SAR tables.
	clamshellSAR, err := getValuesFromASLWithKey(decodedSSDT, "EWRD")
	if err != nil {
		s.Error("Unable to find SAR values, does your device support SAR: ", err)
	}

	clamshellLimits := sarLimits{
		HardMax: clamshellHardMax,
		HardMin: clamshellHardMin,
		SoftMax: clamshellSoftMax,
		SoftMin: clamshellSoftMin,
	}
	// Get and verify the real Clamshell SAR values.
	clamshellRealSARValues, clamshellCompliance := parseAndVerifySARValues(clamshellSAR[3:13], clamshellLimits)
	s.Logf("Clamshell SAR table: %.3f", clamshellRealSARValues)
	switch clamshellCompliance {
	case outsideHardLimits:
		s.Error("Clamshell SAR values exceed limits, requires manual approval")
	case outsideSoftLimits:
		s.Log("WARNING: Clamshell SAR values are near allowable limits")
	default:
		s.Log("Clamshell SAR values are within allowable limits")
	}

}
