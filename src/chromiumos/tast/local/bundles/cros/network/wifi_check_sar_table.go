// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/netiface"
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

// sarTableType is an enum that accounts for the different kinds of SAR tables
// defined by Intel WiFi. We use the general names "profileA" and "profileB" to
// avoid propogating the confusing WRDS and EWRD syntax.
// It could be extended to provide support for WGDS or multiple sets within EWRD.
type sarTableType int

const (
	profileA sarTableType = iota // WRDS
	profileB                     // EWRD
	// TODO(kglund): add support for WGDS
)

// Information about dynamic SAR tables and the relevant acronyms can be found on
// the Chrome OS partner site:
// https://chromeos.google.com/partner/dlm/docs/connectivity/wifidyntxpower.html

// getWifiVendorID returns the vendor ID of the given wireless network interface,
// or returns an error on failure.
func getWifiVendorID(ctx context.Context, netIf string) (string, error) {
	devicePath := filepath.Join("/sys/class/net", netIf, "device")

	vendorID, err := ioutil.ReadFile(filepath.Join(devicePath, "vendor"))
	if err != nil {
		return "", errors.Wrapf(err, "get device %v: failed to get vendor ID", netIf)
	}
	return strings.TrimSpace(string(vendorID)), nil
}

// getSARTableFromASL parses ASL formatted data and returns
// the array of integers from the body of the section labeled with the given key.
// TODO(kglund) unit test this function
func getSARTableFromASL(data []byte, tableType sarTableType) ([]int64, error) {
	dataString := string(data)
	// Remove spaces and newlines from from data to make parsing easier.
	dataString = strings.Replace(dataString, "\n", "", -1)
	dataString = strings.Replace(dataString, " ", "", -1)

	// Try to find the requested table within the data.
	keyIndex := -1
	// The actual requested SAR tables are contained within a subset of the full table
	// in SSDT. tableIndices designates the start and end indices of this subtable.
	var tableKey string
	var tableIndices []int
	var tableName string
	switch tableType {
	case profileA:
		tableKey = "WRDS"
		tableName = "PROFILE_A"
		tableIndices = []int{2, 12}
	case profileB:
		tableKey = "EWRD"
		tableName = "PROFILE_B"
		tableIndices = []int{3, 13}
	}
	keyIndex = strings.Index(dataString, "Name("+tableKey+",Package")
	if keyIndex == -1 {
		return nil, errors.New("could not find " + tableName + " SAR table with key " + tableKey + " in SSDT")
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
	// Return the designated subtable from the full parsed SSDT table.
	return intValues[tableIndices[0]:tableIndices[1]], nil
}

// verifyTable checks the table of type sarTableType contained within decodedSSDT
// against a set of SAR limits. These limits serve as a sanity check for the SAR
// and are not based on a true regulatory standard. The test will fail if the SSDT
// provided does not contain SAR tables.
func verifyTable(decodedSSDT []byte, tableType sarTableType, s *testing.State) {
	const (
		// These values represent the allowable SAR limits
		hardMax = 22.0
		softMax = 20.0
		hardMin = 6.0
		softMin = 8.0
	)

	// There is a special case for SAR tables that indicates an unused or no-op
	// table. These tables are encoded with the value 255 (oxFF) in every index.
	// Such tables are handled and accpeted specifically by this test.

	tableName := ""
	switch tableType {
	case profileA:
		tableName = "PROFILE_A"
	case profileB:
		tableName = "PROFILE_B"
	}

	sarTable, err := getSARTableFromASL(decodedSSDT, tableType)
	if err != nil {
		s.Fatal("Unable to find SAR table: ", err)
	}

	// SAR values are stored in their raw form as ints, which are decoded here
	// into the floats they represent.
	var realSARValues []float64
	exceedsSoftLimits := false
	exceedsHardLimits := false
	// Check for no-op table, which is encoded as a table with 255 in each index.
	isNoOpTable := true
	for _, val := range sarTable {
		if val != 255 {
			isNoOpTable = false
		}
		// Actual SAR values are 1/8 * the stored ints.
		realSARValue := float64(val) / 8.0
		if realSARValue < softMin || realSARValue > softMax {
			exceedsSoftLimits = true
		}
		if realSARValue < hardMin || realSARValue > hardMax {
			exceedsHardLimits = true
		}
		realSARValues = append(realSARValues, realSARValue)
	}
	s.Logf("%v SAR table: %.3f", tableName, realSARValues)
	if isNoOpTable {
		s.Logf("%v is a no-op table, meaning it will not be used", tableName)
		return
	}
	if exceedsHardLimits {
		s.Errorf("%v SAR values exceed limits, requires manual approval", tableName)
		return
	}
	if exceedsSoftLimits {
		s.Logf("WARNING: %v SAR values are near allowable limits", tableName)
		return
	}
	s.Logf("%v SAR values are within allowable limits", tableName)
}

func WifiCheckSARTable(ctx context.Context, s *testing.State) {
	const (
		// SSDT (Secondary System Description Table) contains SAR data
		// in encoded binary format.
		pathToSSDT = "/sys/firmware/acpi/tables/SSDT"
		// Vendor ID for Intel WiFi.
		intelVendorID = "0x8086"
	)
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Verify that the DUT uses Intel WiFi.
	netIf, err := netiface.WifiInterface(ctx, manager, time.Duration(2)*time.Second)
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
	defer os.Remove(pathToDecodedSSDT)
	if err != nil {
		s.Fatal("SSDT decoding failed: ", err)
	}

	// Write the decoded SSDT table to an output file.
	ssdtOut, err := os.Create(filepath.Join(s.OutDir(), "decodedSSDT"))
	defer ssdtOut.Close()
	ssdtOut.Write(decodedSSDT)

	// profileA retrieves WRDS table which stores "static" SAR table.
	verifyTable(decodedSSDT, profileA, s)
	// profileB retrieves EWRD table which stores "dynamic" SAR tables.
	verifyTable(decodedSSDT, profileB, s)
}
