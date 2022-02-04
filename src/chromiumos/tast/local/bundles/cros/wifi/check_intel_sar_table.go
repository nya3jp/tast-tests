// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckIntelSARTable,
		Desc: "Runs a preliminary check on device SAR tables for devices with Intel WiFi",
		Contacts: []string{
			"kglund@google.com",               // Author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func", "wificell_dut_validation", "group:firmware", "firmware_ec", "group:labqual"},
		// NB: The WifiIntel dependency tracks a manually maintained list of devices.
		// If the test is skipping when it should run or vice versa, check the hwdep
		// to see if your board is incorrectly included/excluded.
		HardwareDeps: hwdep.D(hwdep.WifiIntel(),
			// Eve has a unique SAR table configuration which makes it impossible to
			// verify with this test, so we skip all versions of this test on eve.
			// See b/181055964 for more details.
			hwdep.SkipOnModel("eve")),
	})
}

// sarTableType is an enum that accounts for the different kinds of SAR tables
// defined by Intel WiFi. We use the general names "profileA" and "profileB" to
// avoid propogating the confusing WRDS and EWRD syntax.
type sarTableType int

const (
	profileA sarTableType = iota // WRDS
	profileB                     // EWRD
)

// geoSARTable stores a Geo SAR table in units of 0.125 dBm.
type geoSARTable struct {
	version        int64
	max2g          int64
	chainAOffset2g int64
	chainBOffset2g int64
	max5g          int64
	chainAOffset5g int64
	chainBOffset5g int64
	max6g          int64
	chainAOffset6g int64
	chainBOffset6g int64
}

// For the sake of clarity, we convert the the Geo SAR tables to units of 1 dBm
// before printing.
func (table geoSARTable) String() string {
	v2Part := ""
	if table.version == 2 {
		v2Part = fmt.Sprintf(" %.3f %.3f %.3f",
			float64(table.max6g)/8.0, float64(table.chainAOffset6g)/8.0,
			float64(table.chainBOffset6g)/8.0)
	}
	return fmt.Sprintf("{%.3f %.3f %.3f %.3f %.3f %.3f%v}",
		float64(table.max2g)/8.0, float64(table.chainAOffset2g)/8.0,
		float64(table.chainBOffset2g)/8.0, float64(table.max5g)/8.0,
		float64(table.chainAOffset5g)/8.0, float64(table.chainBOffset5g)/8.0,
		v2Part)
}

const (
	// These values represent the allowable SAR limits in units of 1 dBm.
	sarHardMax = 22.0
	sarSoftMax = 20.0
	sarHardMin = 4.0
	sarSoftMin = 6.0
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

// getRawSARValuesAndCheckVersion returns the raw SAR values contained in data
// under the given tableKey and the version of the format of the SAR table.
// If the table is not found, a nil table (and an invalid version number)
// will be returned without an error, since it is not necessarily an error for a table to be missing.
// If we encounter an error while parsing the table, or the version number is not valid,
// return a nil table and an invalid version number alongside the error itself.
func getRawSARValuesAndCheckVersion(data []byte, tableKey string, validVersions []int64) ([]int64, int64, error) {
	dataString := string(data)
	// Remove spaces and newlines from from data to make parsing easier.
	dataString = strings.Replace(dataString, "\n", "", -1)
	dataString = strings.Replace(dataString, " ", "", -1)

	// Try to find the Geo SAR table within the data.
	keyIndex := strings.Index(dataString, "Name("+tableKey+",Package")
	if keyIndex == -1 {
		return nil, -1, nil
	}

	// The fist "{" character denotes the beginning of the data package descriptor.
	startIndex := strings.Index(dataString[keyIndex:], "{") + keyIndex + 1
	sarVersion := dataString[startIndex : startIndex+strings.Index(dataString[startIndex:], ",")]
	intVersion, err := strconv.ParseInt(sarVersion, 0, 64)
	if err != nil {
		return nil, -1, err
	}
	validVersion := false
	for _, version := range validVersions {
		if intVersion == version {
			validVersion = true
			break
		}
	}
	if !validVersion {
		return nil, -1, errors.Errorf("invalid SAR version number %x for table %s", intVersion, tableKey)
	}
	// The second "{" character denotes the beginning of the actual SAR data.
	startIndex = strings.Index(dataString[startIndex:], "{") + startIndex + 1
	endIndex := strings.Index(dataString[startIndex:], "}") + startIndex
	values := strings.Split(dataString[startIndex:endIndex], ",")
	var intValues []int64
	for _, val := range values {
		intVal, err := strconv.ParseInt(val, 0, 64)
		if err != nil {
			return nil, -1, errors.Wrap(err, "invalid ASL format or key")
		}
		intValues = append(intValues, intVal)
	}
	// Ensure that the table is a WiFi table.
	domainType := int(intValues[0])
	if domainType != 0x07 {
		// Domain Type must be 0x07 (WiFi Core).
		return nil, -1, errors.Errorf("invalid Domain Type 0x%02X", domainType)
	}
	return intValues, intVersion, nil
}

// getGeoSARTablesFromASL parses ASL formatted data and returns an array of
// geoSARTable structs derived from the body of the WGDS section.
// If the WGDS section is not found, return nil. If the parsing of the ASL data
// fails, return an error.
func getGeoSARTablesFromASL(data []byte) ([]geoSARTable, error) {
	// Below is an example of the format for the ASL data for a Geo SAR (WGDS)
	// table.
	//
	// Name (WGDS, Package (0x02)
	//            {
	//                0x00000000,
	//                Package (0x13)
	//                {
	//                    0x00000007,
	//                    0x98,
	//                    0x00,
	//                    0x00,
	//                    0x98,
	//                    0x00,
	//                    0x00,
	//                    0x78,
	//                    0x00,
	//                    0x00,
	//                    0x80,
	//                    0x10,
	//                    0x10,
	//                    0x78,
	//                    0x00,
	//                    0x00,
	//                    0x80,
	//                    0x10,
	//                    0x10
	//                }
	//            })
	//
	validGeoSARVersions := []int64{0x00, 0x02}
	values, version, err := getRawSARValuesAndCheckVersion(data, "WGDS", validGeoSARVersions)
	if values == nil {
		// If the Geo table was not found, err will be nil.
		return nil, err
	}

	expectedNumValues := 19
	if version == 2 {
		expectedNumValues = 28
	}
	if len(values) != expectedNumValues {
		return nil, errors.Errorf("Geo SAR table: got %d values; want %d", len(values), expectedNumValues)
	}

	var geoTables []geoSARTable
	offset := 6
	if version == 2 {
		offset = 9
	}
	// Parse out the Geo SAR values.
	for i := 0; i < 3; i++ {
		start := (i * offset) + 1
		currentTable := geoSARTable{
			version:        version,
			max2g:          values[start],
			chainAOffset2g: values[start+1],
			chainBOffset2g: values[start+2],
			max5g:          values[start+3],
			chainAOffset5g: values[start+4],
			chainBOffset5g: values[start+5],
			max6g:          255,
			chainAOffset6g: 0,
			chainBOffset6g: 0,
		}
		if version == 2 {
			currentTable.max6g = values[start+6]
			currentTable.chainAOffset6g = values[start+7]
			currentTable.chainBOffset6g = values[start+8]
		}
		geoTables = append(geoTables, currentTable)
	}
	return geoTables, nil
}

// expectedTableLength returns what the spec says should be the length
// of the WRDS or EWRD table, depending on the revision of those tables.
func expectedTableLength(tableType sarTableType, version int64) int {
	switch tableType {
	case profileA: // WRDS
		switch version {
		case 0:
			// 2 chains (A and B), 5 entries each per table 3-5 of the spec.
			return 2 + 2*5
		case 1:
			// 2 chains (A and B), 11 entries each per table 3-6 of the spec.
			return 2 + 2*11
		case 2:
			// 4 chains (A, B, CDB_A and CDB_B), 11 entries each per table 3-6 of the spec.
			return 2 + 4*11
		}
	case profileB: // EWRD
		switch version {
		case 0:
			// 2 chains (A and B), 3 extra sets, 5 entries per set per table 3-5 of the spec.
			return 3 + 2*3*5
		case 1:
			// 2 chains (A and B), 3 extra sets, 11 entries each per table 3-6 of the spec.
			return 3 + 2*3*11
		case 2:
			// 4 chains (A, B, CDB_A and CDB_B), 3 extra sets, 11 entries each per table 3-6 of the spec.
			return 3 + 4*3*11
		}
	}
	return 0
}

// getSARTableFromASL parses ASL formatted data and returns
// the array of integers from the body of the section labeled with the given key.
// Returns nil if dynamic SAR table is missing or disabled as it is a valid case.
// TODO(kglund) unit test this function
func getSARTableFromASL(data []byte, tableType sarTableType, s *testing.State) ([]int64, error) {
	// Below is an example of the format for the ASL data being parsed.
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
		// EWRD may contain additional tables, but ChromeOS only looks at the first
		// two (high-power and low-power), so we ignore  here.
		tableKey = "EWRD"
		tableName = "PROFILE_B"
		tableIndices = []int{3, 13}
	}
	validSARVersions := []int64{0x00, 0x01, 0x02}
	values, version, err := getRawSARValuesAndCheckVersion(data, tableKey, validSARVersions)
	if err != nil {
		return nil, err
	}
	if values == nil {
		// Missing dynamic SAR table.
		return nil, nil
	}
	enabled := values[1]
	switch enabled {
	case 0:
		// There is a SAR table but it's "disabled". Don't check the validity
		// of the raw values of the table since those will be ignored.
		s.Logf("Table %q is present but disabled", tableKey)
		return nil, nil
	case 1:
		// SAR table is enabled, proceed to validation.
	default:
		return nil, errors.Errorf("invalid 'enabled' setting 0x%02X for table %q",
			enabled, tableName)

	}
	if version == 1 || version == 2 {
		switch tableType {
		case profileA:
			// We do not support CDB, ignore those tables. Only look at SAR tables
			// for Chain A and Chain B.
			tableIndices = []int{2, 24}
		case profileB:
			extraSets := int(values[2])
			// The table has at least 1 set of SAR settings, |extraSets|
			// indicates how many optional sets are present in the table.
			tableIndices = []int{3, 3 + 11*(1+extraSets)}
		}
	}

	tableLength := expectedTableLength(tableType, version)
	// tableIndices[1] should be the length of the array.
	if len(values) != tableLength {
		return nil, errors.Errorf("table %v is malformed; got length %d, want %d",
			tableName, len(values), tableLength)
	}
	return values[tableIndices[0]:tableIndices[1]], nil

}

// verifyAndGetGeoTables checks the Geo SAR tables contained within decodedSSDT and
// returns an array of geoSARTable structs. This function performs a validity check
// to ensure that none of the "max power" fields of the tables is below the minimum
// allowable power. If the Geo SAR tables don't exist, this function logs that fact
// and returns nil. If there is an error parsing the Geo SAR tables, this function
// reports the error and returns nil.
// The Geo offsets themselves are only relevant in the context of the base SAR
// values to which they apply, so they are not directly tested by this function.
func verifyAndGetGeoTables(decodedSSDT []byte, s *testing.State) []geoSARTable {
	geoSARTables, err := getGeoSARTablesFromASL(decodedSSDT)
	if err != nil {
		s.Error("Error occured when parsing Geo SAR (WGDS) table: ", err)
		return nil
	}
	if geoSARTables == nil {
		s.Log("No Geo SAR (WGDS) table found")
		return nil
	}
	s.Log("Geo SAR (WGDS) tables: ", geoSARTables)
	for _, table := range geoSARTables {
		if table.max2g < sarHardMin || table.max5g < sarHardMin || table.max6g < sarHardMin {
			s.Error("Geo SAR table found with max power field below the minimum allowed power")
		}
	}
	return geoSARTables
}

// verifyTable checks the table of type sarTableType contained within decodedSSDT
// against a set of SAR limits. These limits serve as a validity check for the SAR
// and are not based on a true regulatory standard. The test will pass if the SSDT
// provided does not contain SAR tables.
func verifyTable(decodedSSDT []byte, tableType sarTableType, geoTables []geoSARTable, s *testing.State) {
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

	sarTable, err := getSARTableFromASL(decodedSSDT, tableType, s)
	if err != nil {
		s.Fatal("Error parsing SAR table: ", err)
	}
	if sarTable == nil {
		// If no dynamic SAR tables are present, the test should pass automatically.
		s.Logf("No %s SAR table found", tableName)
		return
	}

	// SAR values are stored in their raw form as ints, which are decoded here
	// into the floats they represent.
	var realSARValues []float64
	exceedsSoftLimits := false
	exceedsHardLimits := false
	// Check for no-op table, which is encoded as a table with 255 in each index.
	isNoOpTable := true
	// Check that base SAR values are within allowable limits.
	for _, val := range sarTable {
		if val != 255 {
			isNoOpTable = false
		}
		// Actual SAR values are 1/8 * the stored ints.
		realSARValue := float64(val) / 8.0
		if realSARValue < sarSoftMin || realSARValue > sarSoftMax {
			exceedsSoftLimits = true
		}
		if realSARValue < sarHardMin || realSARValue > sarHardMax {
			exceedsHardLimits = true
		}
		realSARValues = append(realSARValues, realSARValue)
	}
	s.Logf("%v SAR table: %.3f", tableName, realSARValues)
	if isNoOpTable {
		s.Logf("%v is a no-op table, meaning it will not be used", tableName)
		return
	}

	// If we have Geo SAR tables, check that the SAR values do not exceed allowable
	// limits after the relevant offsets have been applied.
	if geoTables != nil {
		for index, realSARValue := range realSARValues {
			for _, geoTable := range geoTables {
				var geoOffset int64
				if geoTable.version == 0 {
					// SAR table format: [0 = 2G_A, 1-4 = 5G_A, 5=2G_B, 6-9=5G_B]
					if index == 0 {
						geoOffset = geoTable.chainAOffset2g
					} else if index < 5 {
						geoOffset = geoTable.chainAOffset5g
					} else if index == 5 {
						geoOffset = geoTable.chainBOffset2g
					} else {
						geoOffset = geoTable.chainBOffset5g
					}
				} else if geoTable.version == 2 {
					// SAR table format: [0 = 2G_A, 1-5 = 5G_A, 6-10 = 6G_A, 11=2G_B, 12-16=5G_B, 17-21 = 6G_B]
					if index == 0 {
						geoOffset = geoTable.chainAOffset2g
					} else if index < 6 {
						geoOffset = geoTable.chainAOffset5g
					} else if index < 11 {
						geoOffset = geoTable.chainAOffset6g
					} else if index == 11 {
						geoOffset = geoTable.chainBOffset2g
					} else if index < 17 {
						geoOffset = geoTable.chainBOffset5g
					} else if index < 21 {
						geoOffset = geoTable.chainBOffset6g
					}
				}
				// Actual Geo SAR values are 1/8 * the stored ints.
				realGeoOffset := float64(geoOffset) / 8.0
				geoAdjustedSARValue := realSARValue + realGeoOffset
				if geoAdjustedSARValue < sarSoftMin || geoAdjustedSARValue > sarSoftMax {
					exceedsSoftLimits = true
				}
				if geoAdjustedSARValue < sarHardMin || geoAdjustedSARValue > sarHardMax {
					exceedsHardLimits = true
				}
			}
		}
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

func CheckIntelSARTable(ctx context.Context, s *testing.State) {
	const (
		// Vendor ID for Intel WiFi.
		intelVendorID = "0x8086"
	)
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Verify that the DUT uses Intel WiFi.
	netIf, err := shill.WifiInterface(ctx, manager, time.Duration(2)*time.Second)
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

	var pathToSSDT string
	for _, path := range []string{
		// SSDT (Secondary System Description Table) contains SAR data
		// in encoded binary format. May show up at different paths
		// depending on the platform.
		"/sys/firmware/acpi/tables/SSDT",
		"/sys/firmware/acpi/tables/SSDT1",
	} {
		if _, err := os.Stat(path); err == nil {
			s.Log("Found SSDT at: ", path)
			pathToSSDT = path
			break
		} else if !os.IsNotExist(err) {
			s.Fatalf("Stat(%q) failed: %v", path, err)
		}
	}
	if pathToSSDT == "" {
		s.Fatal("Failed to find SSDT path")
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

	geoTables := verifyAndGetGeoTables(decodedSSDT, s)
	// profileA retrieves WRDS table which stores "static" SAR table.
	verifyTable(decodedSSDT, profileA, geoTables, s)
	// profileB retrieves EWRD table which stores "dynamic" SAR tables.
	verifyTable(decodedSSDT, profileB, geoTables, s)
}
