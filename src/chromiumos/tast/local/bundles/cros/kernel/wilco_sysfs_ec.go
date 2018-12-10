// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoSysfsEC,
		Desc:         "Tests sysfs interface for Wilco Embedded Controller",
		SoftwareDeps: []string{"wilco_ec"},
		Attr:         []string{"informational"},
	})
}

const wilcoECSysfsDir = "/sys/bus/platform/devices/GOOG000C:00/"

type wilcoECPerms int

const (
	wilcoECRW wilcoECPerms = iota
	wilcoECWO
	wilcoECRO
)

func tryWrite(s *testing.State, path, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0000)
	if err != nil {
		s.Fatalf("Failed to write %q to %s: %v", data, path, err)
	}
}

func tryFailWrite(s *testing.State, path, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0000)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully wrote %q to %s", data, path)
	}
}

func tryRead(s *testing.State, path string) string {
	res, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatalf("Failed to read from %s: %v", path, err)
	}
	return string(res)
}

func tryFailRead(s *testing.State, path string) {
	res, err := ioutil.ReadFile(path)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully read %q from %s", string(res), path)
	}
}

func testBoolSetting(s *testing.State, path string, perms wilcoECPerms) {
	good := []string{"1", "y", "Y", "yEs", "Ye gdfgdf",
		"0", "n", "N", "nO", "0 111 222 333"}
	for _, arg := range good {
		switch perms {
		case wilcoECRW, wilcoECWO:
			tryWrite(s, path, arg)
		case wilcoECRO:
			tryFailWrite(s, path, arg)
		default:
			s.Fatalf(`Unsupported permissions: %d`, perms)
		}
	}

	bad := []string{"2", "a", "-1", "sdfgdsf"}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

func boolSetGetTest(s *testing.State, path string) {
	const crashMsg = "Read %q from %s after writing %q; wanted %q"

	tryWrite(s, path, "0")
	if res := tryRead(s, path); res != "0\n" {
		s.Fatalf(crashMsg, res, path, "0", "0\\n")
	}

	tryWrite(s, path, "1")
	if res := tryRead(s, path); res != "1\n" {
		s.Fatalf(crashMsg, res, path, "1", "1\\n")
	}

	tryWrite(s, path, "0")
	if res := tryRead(s, path); res != "0\n" {
		s.Fatalf(crashMsg, res, path, "0", "0\\n")
	}
}

func testABCParsing(s *testing.State, path string) {
	good := []string{"0 0 0 0", "0013 15 8 45"}
	for _, arg := range good {
		tryWrite(s, path, arg)
	}

	bad := []string{
		"0 0 0",
		"13 15 8 46",
		"13 15 8 60",
		"sdfg",
		"-1 0 0 0",
	}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

// Shared between testPeakshift() and testABC()
// Assumes there is one attribute for each day of the week contained in
// the directory dir, and for each of these writes input, and then expects
// to read expected from it.
func testWeekdayRW(s *testing.State, dir, input, expected string) {
	for dowNum := time.Sunday; dowNum <= time.Saturday; dowNum++ {
		dow := strings.ToLower(dowNum.String())
		path := filepath.Join(dir, dow)
		tryWrite(s, path, input)
		if res := tryRead(s, path); res != expected {
			s.Fatalf("Got %q from reading %s; wanted %q", res, path, expected)
		}
	}
}

func testPeakshiftParsing(s *testing.State, path string) {
	good := []string{" 0 0 0 0 0 0   ", "13 15 8 45 0023 30"}
	for _, arg := range good {
		tryWrite(s, path, arg)
	}

	bad := []string{
		"0 0 0",
		"13 15 8 45 23 29",
		"24 15 8 45 23 30",
		"sdfg",
		"-1 0 0 0 0 0",
	}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

// Takes a string representation of a number, and returns another string
// representation of the number, but left-zero-padded to two digits.
// Throws error if input is not parseable to range [0, 99]
// ex: "0000007" -> "07", "45" -> "45", "000342" -> Error
func rePadNumber(s *testing.State, numString string) string {
	numString = strings.TrimSpace(numString)
	num, err := strconv.ParseInt(numString, 10, 64)
	if err != nil {
		s.Fatalf("Failed to parse %q to int64: %v", numString, err)
	}
	if num < 0 || num > 99 {
		s.Fatalf("Int must be in [0, 99], got %d", num)
	}
	return fmt.Sprintf("%02d", num)
}

func testPeakshiftBattThresh(s *testing.State) {
	path := filepath.Join(wilcoECSysfsDir, "properties/peakshift/battery_threshold")

	good := []string{"15", "50", "00036"}
	for _, arg := range good {
		tryWrite(s, path, arg)
		res := tryRead(s, path)
		arg = rePadNumber(s, arg)
		if exp := arg + "\n"; res != exp {
			s.Fatalf("Read %q from %s; wanted %q", res, path, exp)
		}
	}

	bad := []string{"-1", "sdfd", "14", "51", "99999"}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

func WilcoSysfsEC(ctx context.Context, s *testing.State) {
	// Test boolean attribute behavior
	testBoolSetting(s, filepath.Join(wilcoECSysfsDir, "properties/global_mic_mute_led"), wilcoECRW)
	testBoolSetting(s, filepath.Join(wilcoECSysfsDir, "properties/wireless_sw_wlan"), wilcoECWO)
	testBoolSetting(s, filepath.Join(wilcoECSysfsDir, "stealth_mode"), wilcoECWO)
	tryFailRead(s, filepath.Join(wilcoECSysfsDir, "stealth_mode")) // ensure stealth_mode is WO

	boolPathsRW := []string{
		"properties/auto_boot_on_trinity_dock_attach",
		"properties/global_mic_mute_led",
		"properties/sign_of_life_kbbl",
		"properties/ext_usb_port_en",
		"properties/ich_azalia_en",
		"properties/fn_lock",
		"properties/nic",
		"properties/peakshift/enable",
		"properties/advanced_battery_charging/enable",
	}
	for _, path := range boolPathsRW {
		boolSetGetTest(s, filepath.Join(wilcoECSysfsDir, path))
	}

	// Test Advanced Battery Charging attribute behavior
	// Advanced Charging Mode allows the user to maximize the battery health.
	// In Advanced Charging Mode the system will use standard charging algorithm and
	// other techniques during non-work hours to maximize battery health.
	// During work hours, an express charge is used. This express charge allows the
	// battery to be charged faster; therefore, the battery is at
	// full charge sooner. For each day the time in which the system will be most
	// heavily used is specified by the start time and the duration.
	// Please read the Common UEFI BIOS Behavioral Specification and
	// BatMan 2 BIOS_EC Specification for more details about this feature.
	//
	// The input buffer must have the format
	// "start_hr start_min duration_hr duration_min"
	// The hour fields must be in the range [0-23], and the minutes must be
	// one of (0, 15, 30, 45). The string must be parseable by sscanf() using the
	// format string "%d %d %d %d %d"
	//
	// An example valid input is "0006 15     23 45",
	// which corresponds to a start time of 6:15 and a duration of 23:45
	//
	// The output buffer will be filled with the format
	// "start_hr start_min duration_hr duration_min"
	// The hour fields will be in the range [0-23], and the minutes will be
	// one of (0, 15, 30, 45). Each number will be zero padded to two characters.
	//
	// An example output is "06 15 23 45",
	// which corresponds to a start time of 6:15 and a duration of 23:45
	dir := filepath.Join(wilcoECSysfsDir, "properties/advanced_battery_charging")
	testABCParsing(s, filepath.Join(dir, "sunday"))

	input := "  1 15    8 00045 "
	expected := "01 15 08 45\n"
	testWeekdayRW(s, dir, input, expected)

	// Test Peakshift attribute behavior
	// Peakshift
	// For each weekday a start and end time to run in Peak Shift mode can be set.
	// During these times the system will run from the battery even if the AC is
	// attached as long as the battery stays above the threshold specified.
	// After the end time specified the system will run from AC if attached but
	// will not charge the battery. The system will again function normally using AC
	// and recharging the battery after the specified Charge Start time.
	//
	// The input buffer must have the format
	// "start_hr start_min end_hr end_min charge_start_hr charge_start_min"
	// The hour fields must be in the range [0-23], and the minutes must be
	// one of (0, 15, 30, 45). The string must be parseable by sscanf() using the
	// format string "%d %d %d %d %d %d".
	//
	// An example valid input is "6 15     009 45 23 0",
	// which corresponds to 6:15, 9:45, and 23:00
	//
	// The output buffer will be filled with the format
	// "start_hr start_min end_hr end_min charge_start_hr charge_start_min"
	// The hour fields will be in the range [0-23], and the minutes will be
	// one of (0, 15, 30, 45). Each number will be zero padded to two characters.	//
	//
	// An example output is "06 15 09 45 23 00",
	// which corresponds to 6:15, 9:45, and 23:00
	dir = filepath.Join(wilcoECSysfsDir, "properties/peakshift")
	testPeakshiftParsing(s, filepath.Join(dir, "sunday"))
	testPeakshiftBattThresh(s)

	input = " 1 15 8     45 23 000000  "
	expected = "01 15 08 45 23 00\n"
	testWeekdayRW(s, dir, input, expected)
}
