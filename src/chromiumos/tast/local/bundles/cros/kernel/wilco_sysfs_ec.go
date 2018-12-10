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

type wilcoEcPermissions int

const (
	wecReadWrite wilcoEcPermissions = iota
	wecWriteOnly
	wecReadOnly
)

func tryWrite(s *testing.State, path, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0000)
	if err != nil {
		s.Fatalf("Failed to write %s to %s: ", data, path, err)
	}
}

func tryFailWrite(s *testing.State, path, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0000)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully wrote %s to %s", data, path)
	}
}

func tryRead(s *testing.State, path string) string {
	res, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatalf("Failed to read from %s: ", path, err)
	}
	return string(res)
}

func tryFailRead(s *testing.State, path string) {
	res, err := ioutil.ReadFile(path)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully read %s from %s: ", string(res), path, err)
	}
}

func testBooleanSetting(s *testing.State, path string, perms wilcoEcPermissions) {
	good := []string{"1", "y", "Y", "yEs", "Ye gdfgdf",
		"0", "n", "N", "nO", "0 111 222 333"}
	for _, arg := range good {
		switch perms {
		case wecReadWrite, wecWriteOnly:
			tryWrite(s, path, arg)
		case wecReadOnly:
			tryFailWrite(s, path, arg)
		default:
			s.Fatalf(`Not a supported wilcoEcPermissions`)
		}
	}

	bad := []string{"2", "a", "-1", "sdfgdsf"}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

func booleanSetGetTest(s *testing.State, path string) {
	const crashMsg = "Wanted %s from GET after SET(%s), got %s"

	tryWrite(s, path, "0")
	if res := tryRead(s, path); res != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", res)
	}

	tryWrite(s, path, "1")
	if res := tryRead(s, path); res != "1\n" {
		s.Fatalf(crashMsg, "1\\n", "1", res)
	}

	tryWrite(s, path, "0")
	if res := tryRead(s, path); res != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", res)
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
// the directory @dir, and for each of these writes @input, and then expects
// to read @expected from it
func testWeekdayRW(s *testing.State, dir, input, expected string) {
	for dowNum := time.Sunday; dowNum <= time.Saturday; dowNum++ {
		dow := strings.ToLower(dowNum.String())
		path := filepath.Join(dir, dow)
		tryWrite(s, path, input)
		if res := tryRead(s, path); res != expected {
			s.Fatalf("Expected %s from reading %s, got %s", expected, path, res)
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
func rePadNumber(s *testing.State, numString string, padding int) string {
	numString = strings.TrimSpace(numString)
	num, err := strconv.ParseInt(numString, 10, 64)
	if err != nil {
		s.Fatalf("Failed to parse %s to int64", numString)
	}
	if num < 0 || num > 99 {
		s.Fatalf("Int must be in [0, 99], got %d", num)
	}
	format := fmt.Sprintf("%%0%dd", padding) // create "%0[padding]d"
	return fmt.Sprintf(format, num)
}

func testPeakshiftBattThresh(s *testing.State) {
	path := filepath.Join(wilcoECSysfsDir, "properties/peakshift/battery_threshold")

	good := []string{"15", "50", "00036"}
	for _, arg := range good {
		tryWrite(s, path, arg)
		res := tryRead(s, path)
		arg = rePadNumber(s, arg, 2)
		if exp := arg + "\n"; res != exp {
			s.Fatalf("Expected '%s\\n' from reading %s, got %s",
				arg, path, res)
		}
	}

	bad := []string{"-1", "sdfd", "14", "51", "99999"}
	for _, arg := range bad {
		tryFailWrite(s, path, arg)
	}
}

func WilcoSysfsEC(ctx context.Context, s *testing.State) {
	// Test boolean attribute behavior
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "properties/global_mic_mute_led"), wecReadWrite)
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "properties/wireless_sw_wlan"), wecWriteOnly)
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "stealth_mode"), wecWriteOnly)
	tryFailRead(s, filepath.Join(wilcoECSysfsDir, "stealth_mode")) // ensure stealth_mode is WO

	booleanpathibutesRW := []string{
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
	for _, path := range booleanpathibutesRW {
		booleanSetGetTest(s, filepath.Join(wilcoECSysfsDir, path))
	}

	// Test Advanced Battry Charging attribute behavior
	dir := filepath.Join(wilcoECSysfsDir, "properties/advanced_battery_charging")
	testABCParsing(s, filepath.Join(dir, "sunday"))

	input := "  1 15 8 00045 "
	expected := "01 15 08 45\n"
	testWeekdayRW(s, dir, input, expected)

	// Test Peakshift attribute behavior
	dir = filepath.Join(wilcoECSysfsDir, "properties/peakshift")

	testPeakshiftParsing(s, filepath.Join(dir, "sunday"))
	testPeakshiftBattThresh(s)

	input = " 1 15 8 45 23 000000  "
	expected = "01 15 08 45 23 00\n"
	testWeekdayRW(s, dir, input, expected)
}
