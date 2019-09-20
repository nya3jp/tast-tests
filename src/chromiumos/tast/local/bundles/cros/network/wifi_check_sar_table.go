// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// These values represent the allowable SAR limits for clamshell and tablet modes.
const (
	tabletHardMax    = 14.0
	tabletSoftMax    = 16.0
	tabletHardMin    = 6.0
	tabletSoftMin    = 8.0
	clamshellSoftMax = 20.0
	clamshellHardMax = 22.0
	clamshellSoftMin = 12.0
	clamshellHardMin = 10.0
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WifiCheckSarTable,
		Desc: "Runs a preliminary check on device SAR tables",
		Contacts: []string{
			"kglund@google.com",               // Author
			"chromeos-kernel-wifi@google.com", // WiFi team
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wifi"},
	})
}

// getValuesFromASLWithKey parses ASL formatted data and returns
// the array of integers from the body of the section labeled with the given key.
func getValuesFromASLWithKey(data []byte, key string, s *testing.State) ([]int64, error) {
	dataString := string(data)
	// Clear spaces and newlines from from data to make parsing easier.
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
	intValues := []int64{}
	for _, val := range values {
		intVal, err := strconv.ParseInt(val, 0, 64)
		if err != nil {
			return nil, errors.New("invalid ASL format or key")
		}
		intValues = append(intValues, intVal)
	}
	return intValues, nil
}

func WifiCheckSarTable(ctx context.Context, s *testing.State) {
	// SSDT (Secondary System Description Table) contains SAR data
	// in encoded binary format.
	pathToSSDT := "/sys/firmware/acpi/tables/SSDT"
	SSDTRaw, err := ioutil.ReadFile(pathToSSDT)
	if err != nil {
		s.Fatalf("Could not read SSDT data: %s", err)
	}
	// Write encoded SSDT data to temp file.
	tmpSSDT, err := ioutil.TempFile("/tmp", "tempSSDT")
	if err != nil {
		s.Fatalf("Could not create temp file for SSDT: %s", err)
	}
	defer os.Remove(tmpSSDT.Name())
	if _, err := tmpSSDT.Write(SSDTRaw); err != nil {
		s.Fatalf("Could not write to temp SSDT file: %s", err)
	}

	// Use iasl to decode the data into ASL format.
	cmd := testexec.CommandContext(ctx, "iasl", "-d", tmpSSDT.Name())
	err = cmd.Run()
	if err != nil {
		s.Error("Could not run iasl on DUT")
	}
	// Read in the decoded table.
	pathToDecodedSSDT := tmpSSDT.Name() + ".dsl"
	decodedSSDT, err := ioutil.ReadFile(pathToDecodedSSDT)
	if err != nil {
		s.Fatalf("SSDT decoding failed: %s", err)
	}

	// WRDS stores tablet mode SAR tables.
	tabletSAR, err := getValuesFromASLWithKey(decodedSSDT, "WRDS", s)
	if err != nil {
		s.Error("Unable to find SAR values, does your device support SAR?")
	}
	var tabletSARValues []float64
	tabletExceedsSoftLimits := false
	tabletExceedsHardLimits := false
	for _, val := range tabletSAR[2:] {
		// Actual SAR values are 1/8 * the stored ints.
		realSARValue := float64(val) / 8
		tabletSARValues = append(tabletSARValues, realSARValue)
		if realSARValue < tabletSoftMin || realSARValue > tabletSoftMax {
			tabletExceedsSoftLimits = true
		}
		if realSARValue < tabletHardMin || realSARValue > tabletHardMax {
			tabletExceedsHardLimits = true
		}
	}
	if tabletExceedsHardLimits {
		s.Error("Tablet SAR values exceed limits, requires manual approval")
	} else if tabletExceedsSoftLimits {
		s.Log("WARNING: Tablet SAR values are near allowable limits")
	}
	s.Logf("Tablet SAR table: %.3f", tabletSARValues)

	// EWRD stores clamshell mode SAR tables.
	clamshellSAR, err := getValuesFromASLWithKey(decodedSSDT, "EWRD", s)
	if err != nil {
		s.Error("Unable to find SAR values, does your device support SAR?")
	}
	var clamshellSARValues []float64
	clamshellExceedsSoftLimits := false
	clamshellExceedsHardLimits := false
	for _, val := range clamshellSAR[3:13] {
		// Actual SAR values are 1/8 * the stored ints.
		realSARValue := float64(val) / 8
		clamshellSARValues = append(clamshellSARValues, realSARValue)
		if realSARValue < clamshellSoftMin || realSARValue > clamshellSoftMax {
			clamshellExceedsSoftLimits = true
		}
		if realSARValue < clamshellHardMin || realSARValue > clamshellHardMax {
			clamshellExceedsHardLimits = true
		}
	}
	if clamshellExceedsHardLimits {
		s.Error("Clamshell SAR values exceed limits, requires manual approval")
	} else if clamshellExceedsSoftLimits {
		s.Log("WARNING: Clamshell SAR values are near allowable limits")
	}
	s.Logf("Clamshell SAR table: %.3f", clamshellSARValues)
}
