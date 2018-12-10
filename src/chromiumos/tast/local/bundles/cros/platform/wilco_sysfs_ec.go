// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

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
		Desc:         "Test sysfs interface for Wilco Embedded Controller",
		SoftwareDeps: []string{},
		Attr:         []string{"informational"},
	})
}

const wilcoECSysfsDir = "/sys/bus/platform/devices/GOOG000C:00/"

func tryWrite(s *testing.State, attr string, inp string) {
	err := ioutil.WriteFile(attr, []byte(inp), 0000)
	if err != nil {
		s.Fatalf("Failed to write '%s' to '%s': ", inp, attr, err)
	}
}

func tryFailWrite(s *testing.State, attr string, inp string) {
	err := ioutil.WriteFile(attr, []byte(inp), 0000)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully wrote '%s' to '%s'", inp, attr)
	}
}

func tryRead(s *testing.State, attr string) string {
	res, err := ioutil.ReadFile(attr)
	if err != nil {
		s.Fatalf("Failed to read from '%s': ", attr, err)
	}
	return string(res)
}

func tryFailRead(s *testing.State, attr string) {
	res, err := ioutil.ReadFile(attr)
	if err == nil {
		s.Fatalf("Wanted to fail, but successfully read '%s' from '%s': ", string(res), attr, err)
	}
}

func testBooleanSetting(s *testing.State, attr, permissions string) {
	good := []string{"1", "y", "Y", "yEs", "Ye gdfgdf",
		"0", "n", "N", "nO", "0 111 222 333"}
	bad := []string{"2", "a", "-1", "sdfgdsf"}

	for _, arg := range good {
		switch permissions {
		case "RW", "WO":
			tryWrite(s, attr, arg)
		case "RO":
			tryFailWrite(s, attr, arg)
		default:
			s.Fatalf(`permissions must be one of {"RW", "WO", "RO"}, got "%s"`)
		}
	}
	for _, arg := range bad {
		tryFailWrite(s, attr, arg)
	}
}

func booleanSetGetTest(s *testing.State, attr string) {
	crashMsg := "Wanted '%s' from GET after SET(%s), got %s"
	var res string

	tryWrite(s, attr, "0")
	res = tryRead(s, attr)
	if res != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", res)
	}

	tryWrite(s, attr, "1")
	res = tryRead(s, attr)
	if res != "1\n" {
		s.Fatalf(crashMsg, "1\\n", "1", res)
	}

	tryWrite(s, attr, "0")
	res = tryRead(s, attr)
	if res != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", res)
	}
}

func testBooleans(s *testing.State) {
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "properties/global_mic_mute_led"), "RW")
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "properties/wireless_sw_wlan"), "WO")
	testBooleanSetting(s, filepath.Join(wilcoECSysfsDir, "stealth_mode"), "WO")
	tryFailRead(s, filepath.Join(wilcoECSysfsDir, "stealth_mode")) //ensure stealth_mode is WO

	booleanAttributesRW := []string{
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
	for _, attr := range booleanAttributesRW {
		booleanSetGetTest(s, filepath.Join(wilcoECSysfsDir, attr))
	}
}

func testABCParsing(s *testing.State, attr string) {
	good := []string{"0 0 0 0", "0013 15 8 45"}
	bad := []string{
		"0 0 0",
		"13 15 8 46",
		"13 15 8 60",
		"sdfg",
		"-1 0 0 0",
	}
	for _, arg := range good {
		tryWrite(s, attr, arg)
	}
	for _, arg := range bad {
		tryFailWrite(s, attr, arg)
	}
}

func testABC(s *testing.State) {
	dir := filepath.Join(wilcoECSysfsDir, "properties", "advanced_battery_charging")
	testABCParsing(s, filepath.Join(dir, "sunday"))

	inp := "  1 15 8 00045 "
	expected := "01 15 08 45\n"
	for dowNum := time.Sunday; dowNum <= time.Saturday; dowNum++ {
		DOW := strings.ToLower(dowNum.String())
		attr := filepath.Join(dir, DOW)
		tryWrite(s, attr, inp)
		res := tryRead(s, attr)
		if res != expected {
			s.Fatalf("Expected '%s' from reading '%s', got '%s'", expected, attr, res)
		}
	}
}

func testPeakshiftParsing(s *testing.State, attr string) {
	good := []string{" 0 0 0 0 0 0   ", "13 15 8 45 0023 30"}
	bad := []string{
		"0 0 0",
		"13 15 8 45 23 29",
		"24 15 8 45 23 30",
		"sdfg",
		"-1 0 0 0 0 0",
	}

	for _, arg := range good {
		tryWrite(s, attr, arg)
	}
	for _, arg := range bad {
		tryFailWrite(s, attr, arg)
	}
}

func rePadNumber(s *testing.State, numString string, padding int) string {
	ns2 := strings.TrimSpace(numString)
	num, err := strconv.ParseInt(ns2, 10, 64)
	if err != nil {
		s.Fatalf("Failed to parse '%s' to int64", ns2)
	}
	format := fmt.Sprintf("%%0%dd", padding) //create "%0[padding]d"
	string2 := fmt.Sprintf(format, num)
	return string2
}

func testPeakshiftBattThresh(s *testing.State) {
	attr := filepath.Join(wilcoECSysfsDir, "properties", "peakshift", "battery_threshold")

	for _, arg := range []string{"15", "50", "00036"} {
		tryWrite(s, attr, arg)
		res := tryRead(s, attr)
		arg = rePadNumber(s, arg, 2)
		if res != arg+"\n" {
			s.Fatalf("Expected '%s\\n' from reading '%s', got '%s'",
				arg, attr, res)
		}
	}

	for _, arg := range []string{"-1", "sdfd", "14", "51", "99999"} {
		tryFailWrite(s, attr, arg)
	}
}

func testPeakshift(s *testing.State) {
	dir := filepath.Join(wilcoECSysfsDir, "properties", "peakshift")

	testPeakshiftParsing(s, filepath.Join(dir, "sunday"))
	testPeakshiftBattThresh(s)

	inp := " 1 15 8 45 23 000000  "
	expected := "01 15 08 45 23 00\n"
	for dowNum := time.Sunday; dowNum <= time.Saturday; dowNum++ {
		DOW := strings.ToLower(dowNum.String())
		attr := filepath.Join(dir, DOW)
		tryWrite(s, attr, inp)
		res := tryRead(s, attr)
		if res != expected {
			s.Fatalf("Expected '%s' from reading '%s', got '%s'", expected, attr, res)
		}
	}
}

func WilcoSysfsEC(ctx context.Context, s *testing.State) {
	testBooleans(s)
	testABC(s)
	testPeakshift(s)
}
