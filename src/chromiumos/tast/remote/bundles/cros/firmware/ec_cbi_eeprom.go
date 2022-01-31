// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECCbiEeprom,
		Desc:         "Test that I2C controls can be used to read/write EEPROM, and setting write protect prevents writing",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.ECFeatureCBI()),
	})
}

type eepromData struct {
	wpDecoupled bool
	testData1   string
	testData2   string
	oldData     string
	writeData   string
	newData     string
	i2c         *firmware.I2CLookupInfo
	pageSize    int
	maxBytes    int
	offset      int
}

func ECCbiEeprom(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	pageSize := 8
	maxBytes := 64
	testData1 := strings.TrimRight(strings.Repeat("0xaa ", pageSize), " ")
	testData2 := strings.TrimRight(strings.Repeat("0x55 ", pageSize), " ")

	s.Log("Getting I2C info")
	i2cInfo, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).I2CLookup(ctx)
	if err != nil {
		s.Fatal("Locate chip with ectool: ", err)
	}

	s.Log("Attempting to set 'i2c_mux_en' to 'off'")
	if hasCtrl, err := h.Servo.HasControl(ctx, string(servo.I2CMuxEn)); err != nil {
		s.Fatal("Failed to check if servo has control: ", servo.I2CMuxEn)
	} else if hasCtrl {
		if err := h.Servo.SetOnOff(ctx, servo.I2CMuxEn, servo.Off); err != nil {
			s.Fatalf("Failed to set %q control to %s", servo.I2CMuxEn, servo.Off)
		}
		s.Logf("Control %q is present and was reset", servo.I2CMuxEn)
	} else {
		s.Logf("Control %q does not exist, won't reset", servo.I2CMuxEn)
	}

	data := &eepromData{
		wpDecoupled: false, oldData: "", writeData: "", newData: "",
		i2c: i2cInfo, pageSize: pageSize, maxBytes: maxBytes, offset: 0,
		testData1: testData1, testData2: testData2,
	}

	s.Log("Use ectool gpioget to check if cbi wp is decoupled")
	out, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).GPIOGet(ctx, firmware.ECCbiWp)
	if err != nil {
		s.Log("CBI WP is not decoupled from EC WP")
	} else {
		s.Log("CBI WP is decoupled from EC WP: ", out)
		data.wpDecoupled = true
	}

	s.Log("Read initial data that can be restored")
	initialData, err := readInitialData(ctx, h, s.DUT(), data)
	if err != nil {
		s.Fatal("Failed to read initial data on eeprom: ", err)
	}
	defer func() {
		s.Log("Cleaning up wp status")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			s.Fatal("Failed to disable firmware write protect: ", err)
		}
		if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
			s.Fatal("Failed to disable flashwp: ", err)
		}

		if data.wpDecoupled {
			if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-disable").Output(ssh.DumpLogOnError); err != nil {
				s.Fatal("Failed to disable wp: ", err)
			}
		}

		s.Log("Reboot DUT")
		ms, err := firmware.NewModeSwitcher(ctx, h)
		if err != nil {
			s.Fatal("Failed to create mode switcher: ", err)
		}
		if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
			s.Fatal("Failed to perform mode aware reboot: ", err)
		}

		s.Log("Rewrite initial data")
		if err := reWriteInitialData(ctx, h, s.DUT(), data, initialData); err != nil {
			s.Fatal("Failed to reset eeprom data to original value: ", err)
		}

	}()

	s.Log("Test EEPROM is writable with WP disabled")
	if err := checkEepromWithoutWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp disabled")
	}

	s.Log("Test EEPROM is not writable with WP enabled")
	if err := checkEepromWithWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp enabled")
	}
}

func readInitialData(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) ([]string, error) {
	initialData := make([]string, 0)

	for offset := 0; offset < data.maxBytes; offset += data.pageSize {
		data.offset = offset
		testing.ContextLog(ctx, "Reading from eeprom at offset: ", data.offset)
		readData, err := readEeprom(ctx, d, data)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read eeprom")
		}
		initialData = append(initialData, readData)
	}

	return initialData, nil
}

func reWriteInitialData(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData, initialData []string) error {
	for offset := 0; offset < data.maxBytes; offset += data.pageSize {
		data.offset = offset

		data.writeData = initialData[data.offset/data.pageSize]
		testing.ContextLog(ctx, "Old data to write is: ", data.writeData)

		if err := writeEeprom(ctx, d, data); err != nil {
			return errors.Wrap(err, "failed to write to eeprom")
		}

		newData, err := readEeprom(ctx, d, data)
		if err != nil {
			return errors.Wrap(err, "failed to read eeprom")
		}
		data.newData = newData

		testing.ContextLog(ctx, "Newly written data was: ", newData)

		if data.writeData != data.newData {
			return errors.New("EEPROM data did not get reset to initial data")
		}
	}

	return nil
}

func checkEepromWithWP(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	if err := h.Servo.RunECCommand(ctx, "flashwp enable"); err != nil {
		return errors.Wrap(err, "failed to enable flashwp")
	}

	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		return errors.Wrap(err, "failed to enable firmware write protect")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		return errors.Wrap(err, "failed to perform mode aware reboot")
	}

	if data.wpDecoupled {
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-enable").Output(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to enable wp")
		}
	}

	for offset := 0; offset < data.maxBytes; offset += data.pageSize {
		data.offset = offset

		testing.ContextLog(ctx, "Reading from eeprom at offset: ", data.offset)
		oldData, err := readEeprom(ctx, d, data)
		if err != nil {
			return errors.Wrap(err, "failed to read eeprom")
		}
		data.oldData = oldData

		data.writeData = data.testData1
		if match := regexp.MustCompile(data.testData1).FindStringSubmatch(oldData); match != nil && len(match) > 0 {
			data.writeData = data.testData2
		}

		if err := writeEeprom(ctx, d, data); err == nil {
			return errors.Wrap(err, "write to eeprom unexpectedly succeeded with wp enabled")
		}

		newData, err := readEeprom(ctx, d, data)
		if err != nil {
			return errors.Wrap(err, "failed to read eeprom")
		}
		data.newData = newData

		if data.oldData != data.newData {
			return errors.New("EEPROM data unexpectedly updated with wp enabled")
		}
	}

	return nil
}

func checkEepromWithoutWP(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		return errors.Wrap(err, "failed to disable firmware write protect")
	}

	if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
		return errors.Wrap(err, "failed to disable flashwp")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		return errors.Wrap(err, "failed to perform mode aware reboot")
	}

	if data.wpDecoupled {
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-disable").Output(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to disable wp")
		}
	}

	for offset := 0; offset < data.maxBytes; offset += data.pageSize {
		data.offset = offset

		oldData, err := readEeprom(ctx, d, data)
		if err != nil {
			return errors.Wrap(err, "failed to read eeprom")
		}
		data.oldData = oldData

		data.writeData = data.testData1
		if match := regexp.MustCompile(data.testData1).FindStringSubmatch(oldData); match != nil && len(match) > 0 {
			data.writeData = data.testData2
		}

		if err := writeEeprom(ctx, d, data); err != nil {
			return errors.Wrap(err, "failed to write to eeprom")
		}

		newData, err := readEeprom(ctx, d, data)
		if err != nil {
			return errors.Wrap(err, "failed to read eeprom")
		}
		data.newData = newData

		if data.writeData != data.newData {
			return errors.New("EEPROM data did not update with wp disabled")
		}
	}

	return nil
}

func readEeprom(ctx context.Context, d *dut.DUT, eepromData *eepromData) (string, error) {
	readArgs := []string{
		strconv.Itoa(eepromData.i2c.Port), strconv.Itoa(eepromData.i2c.Address),
		strconv.Itoa(eepromData.pageSize), strconv.Itoa(eepromData.offset),
	}

	testing.ContextLog(ctx, "Using ectool i2cxfer to read at offset: ", eepromData.offset)
	out, err := firmware.NewECTool(d, firmware.ECToolNameMain).I2C(ctx, firmware.I2Cxfer, readArgs...)
	if err != nil {
		return "", errors.Wrap(err, "failed to read eeprom data")
	}

	if match := regexp.MustCompile(`Read bytes: (.+)`).FindStringSubmatch(out); match == nil || len(match) == 0 {
		return "", errors.New("failed to read EEPROM data")
	} else if string(match[1]) == "" {
		return "", errors.Errorf("empty EEPROM at offset %d", eepromData.offset)
	} else {
		return string(match[1]), nil
	}
}

func writeEeprom(ctx context.Context, d *dut.DUT, eepromData *eepromData) error {
	writeArgs := []string{
		strconv.Itoa(eepromData.i2c.Port),
		strconv.Itoa(eepromData.i2c.Address),
		"0", strconv.Itoa(eepromData.offset),
	}
	writeArgs = append(writeArgs, strings.Split(eepromData.writeData, " ")...)

	testing.ContextLog(ctx, "Using ectool i2cxfer to write at offset: ", eepromData.offset)
	_, err := firmware.NewECTool(d, firmware.ECToolNameMain).I2C(ctx, firmware.I2Cxfer, writeArgs...)
	if err != nil {
		return errors.Wrap(err, "failed to write eeprom data")
	}
	return nil
}
