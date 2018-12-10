// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const deviceDir = "/sys/bus/platform/devices/GOOG000C\\:00/"

var daysOfWeek = []string{
	"sunday",
	"monday",
	"tuesday",
	"wednesday",
	"thursday",
	"friday",
	"saturday",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoSysfsEC,
		Desc:         "sysfs interface for Wilco Embedded Controller",
		SoftwareDeps: []string{},
		Attr:         []string{"informational"},
	})
}

func runAndMaybeCrash(ctx context.Context, s *testing.State, testCmd string) []byte {
	cmd := testexec.CommandContext(ctx, "bash", "-c", testCmd)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal(fmt.Sprintf("Failed to run 'bash -c %s'", testCmd), err)
	}
	return out
}

func runAndHopefullyCrash(ctx context.Context, s *testing.State, testCmd string) {
	cmd := testexec.CommandContext(ctx, "bash", "-c", testCmd)
	_, err := cmd.Output()
	if err == nil {
		cmd.DumpLog(ctx)
		MSG := "Successfully ran, but wanted to crash: 'bash -c %s'"
		s.Fatal(fmt.Sprintf(MSG, testCmd), err)
	}
}

func testBooleanSetting(ctx context.Context, s *testing.State,
	attr, permisions string) {
	pos := []string{"1", "y", "Y", "yEs", "Ye gdfgdf"}
	neg := []string{"0", "n", "N", "nO", "0 111 222 333"}
	bad := []string{"2", "a", "-1", "sdfgdsf"}

	for _, arg := range pos {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		switch permisions {
		case "RW", "WO":
			runAndMaybeCrash(ctx, s, testCmd)
		case "RO":
			runAndHopefullyCrash(ctx, s, testCmd)
		}
	}
	for _, arg := range neg {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		switch permisions {
		case "RW", "WO":
			runAndMaybeCrash(ctx, s, testCmd)
		case "RO":
			runAndHopefullyCrash(ctx, s, testCmd)
		}
	}
	for _, arg := range bad {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		runAndHopefullyCrash(ctx, s, testCmd)
	}
}

func booleanSetGetTest(ctx context.Context, s *testing.State, attr string) {
	crashMsg := "Wanted '%s' from GET after SET(%s), got %s"

	testCmd := fmt.Sprintf("echo 0 > %s", attr)
	runAndMaybeCrash(ctx, s, testCmd)
	testCmd = fmt.Sprintf("cat %s", attr)
	res := runAndMaybeCrash(ctx, s, testCmd)
	if string(res) != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", string(res))
	}

	testCmd = fmt.Sprintf("echo 1 > %s", attr)
	runAndMaybeCrash(ctx, s, testCmd)
	testCmd = fmt.Sprintf("cat %s", attr)
	res = runAndMaybeCrash(ctx, s, testCmd)
	if string(res) != "1\n" {
		s.Fatalf(crashMsg, "1\\n", "1", string(res))
	}

	testCmd = fmt.Sprintf("echo 0 > %s", attr)
	runAndMaybeCrash(ctx, s, testCmd)
	testCmd = fmt.Sprintf("cat %s", attr)
	res = runAndMaybeCrash(ctx, s, testCmd)
	if string(res) != "0\n" {
		s.Fatalf(crashMsg, "0\\n", "0", string(res))
	}
}

func testBooleans(ctx context.Context, s *testing.State) {
	DIR := deviceDir + "properties/"
	testBooleanSetting(ctx, s, DIR+"global_mic_mute_led", "RW")
	testBooleanSetting(ctx, s, DIR+"wireless_sw_wlan", "WO")

	booleanAttributesRW := []string{
		"auto_boot_on_trinity_dock_attach",
		"global_mic_mute_led",
		"sign_of_life_kbbl",
		"ext_usb_port_en",
		"ich_azalia_en",
		"fn_lock",
		"nic",
		"peakshift/enable",
		"advanced_battery_charging/enable",
	}
	for _, attr := range booleanAttributesRW {
		booleanSetGetTest(ctx, s, DIR+attr)
	}
}

func testABCParsing(ctx context.Context, s *testing.State, attr string) {
	good := []string{" 0 0 0 0  ", "0013 15 8 45"}
	bad := []string{
		"0 0 0",
		"13 15 8 46",
		"13 15 8 60",
		"sdfg",
		"-1 0 0 0",
		"",
	}
	for _, arg := range good {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		runAndMaybeCrash(ctx, s, testCmd)
	}
	for _, arg := range bad {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		runAndHopefullyCrash(ctx, s, testCmd)
	}
}

func testABC(ctx context.Context, s *testing.State) {
	DIR := deviceDir + "properties/advanced_battery_charging/"
	FMT := "Expected '01 15 08 45\\n' from '%s', got '%s'"
	testABCParsing(ctx, s, DIR+"sunday")

	var testCmd, res string
	for _, DOW := range daysOfWeek {
		testCmd = fmt.Sprintf("echo 1 15 8 45 > %s", DIR+DOW)
		runAndMaybeCrash(ctx, s, testCmd)
		testCmd = fmt.Sprintf("cat %s", DIR+DOW)
		res = string(runAndMaybeCrash(ctx, s, testCmd))
		if res != "01 15 08 45\n" {
			s.Fatalf(FMT, testCmd, res)
		}
	}
}

func testPeakshiftParsing(ctx context.Context, s *testing.State, attr string) {
	good := []string{" 0 0 0 0 0 0   ", "13 15 8 45 0023 30"}
	bad := []string{
		"0 0 0",
		"13 15 8 45 23 29",
		"24 15 8 45 23 30",
		"sdfg",
		"-1 0 0 0 0 0",
	}

	for _, arg := range good {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		runAndMaybeCrash(ctx, s, testCmd)
	}
	for _, arg := range bad {
		testCmd := fmt.Sprintf("echo %s > %s", arg, attr)
		runAndHopefullyCrash(ctx, s, testCmd)
	}
}

func rePadNumber(numString string, padding int) string {
	ns2 := strings.TrimSpace(numString)
	num, _ := strconv.ParseInt(ns2, 10, 64)
	FMT := "%0" + fmt.Sprintf("%d", padding) + "d"
	string2 := fmt.Sprintf(FMT, num)
	return string2
}

func testPeakshiftBattThresh(ctx context.Context, s *testing.State) {
	ATTR := deviceDir + "properties/peakshift/battery_threshold"
	good := []string{"  15", "50  ", "00036"}
	bad := []string{"", "-1", "sdfd", "14", "51", "99999"}
	var testCmd, res string
	for _, arg := range good {
		testCmd = fmt.Sprintf("echo %s > %s", arg, ATTR)
		runAndMaybeCrash(ctx, s, testCmd)
		testCmd = fmt.Sprintf("cat %s", ATTR)
		res = string(runAndMaybeCrash(ctx, s, testCmd))
		arg = rePadNumber(arg, 2)
		if res != arg+"\n" {
			s.Fatalf("Expected '%s\\n' from '%s', got '%s'",
				arg, testCmd, res)
		}
	}
	for _, arg := range bad {
		testCmd := fmt.Sprintf("echo %s > %s", arg, ATTR)
		runAndHopefullyCrash(ctx, s, testCmd)
	}
}

func testPeakshift(ctx context.Context, s *testing.State) {
	DIR := deviceDir + "properties/peakshift/"
	FMT := "Expected '01 15 08 45 23 00\\n' from '%s', got '%s'"

	testPeakshiftParsing(ctx, s, DIR+"sunday")
	testPeakshiftBattThresh(ctx, s)

	var testCmd, res string
	for _, DOW := range daysOfWeek {
		testCmd = fmt.Sprintf("echo 1 15 8 45 23 0 > %s", DIR+DOW)
		runAndMaybeCrash(ctx, s, testCmd)
		testCmd = fmt.Sprintf("cat %s", DIR+DOW)
		res = string(runAndMaybeCrash(ctx, s, testCmd))
		if res != "01 15 08 45 23 00\n" {
			s.Fatalf(FMT, testCmd, res)
		}
	}
}

func WilcoSysfsEC(ctx context.Context, s *testing.State) {
	testBooleans(ctx, s)
	testABC(ctx, s)
	testPeakshift(ctx, s)
}
