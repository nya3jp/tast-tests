// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strconv"
	"time"

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
	writeData   string
	cbiTag      string
	writeSize   int
}

func ECCbiEeprom(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	pageSize := 8
	writeSize := (pageSize + 1) / 2 // + 1 in case page size is made an odd num.
	testCbiTag := "99"

	testData1 := "0xb928de99"
	testData2 := "0xeb79441e"

	s.Log("Test data 1: ", testData1)
	s.Log("Test data 2: ", testData2)

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
		wpDecoupled: false, testData1: testData1, testData2: testData2,
		writeData: "", cbiTag: testCbiTag, writeSize: writeSize,
	}

	s.Log("Use ectool gpioget to check if cbi wp is decoupled")
	out, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).FindBaseGpio(ctx, []firmware.GpioName{firmware.ECCbiWp})
	if err != nil || out == nil {
		s.Log("CBI WP is not decoupled from EC WP")
	} else if val, ok := out[firmware.ECCbiWp]; ok {
		s.Log("CBI WP is decoupled from EC WP: ", val)
		data.wpDecoupled = true
	}

	defer func() {
		s.Log("Cleaning up wp status")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			s.Fatal("Failed to disable firmware write protect: ", err)
		}
		if err := h.Servo.RunECCommand(ctx, "flashwp false"); err != nil {
			s.Fatal("Failed to disable flashwp: ", err)
		}

		// if data.wpDecoupled {
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-disable").Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to disable wp: ", err)
		}
		// }

		s.Log("Reboot DUT")
		ms, err := firmware.NewModeSwitcher(ctx, h)
		if err != nil {
			s.Fatal("Failed to create mode switcher: ", err)
		}
		if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
			s.Fatal("Failed to perform mode aware reboot: ", err)
		}
	}()

	s.Log("Test EEPROM is not writable with WP enabled")
	if err := checkEepromWithWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp enabled: ", err)
	}

	s.Log("Test EEPROM is writable with WP disabled")
	if err := checkEepromWithoutWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp disabled: ", err)
	}

}

func checkEepromWithWP(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	// Remove tag if already exists before enabling wp to test creating new tag.
	testing.ContextLogf(ctx, "Checking if tag %q already exists", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, d, data); err == nil {
		testing.ContextLogf(ctx, "tag %q already exists in CBI, removing it", data.cbiTag)
		if err := removeTagFromEeprom(ctx, d, data); err != nil {
			return errors.Wrap(err, "failed to remove tag from cbi")
		}
	}

	testing.ContextLog(ctx, "Enabling write protect")
	if err := h.Servo.RunECCommand(ctx, "flashwp true"); err != nil {
		return errors.Wrap(err, "failed to enable flashwp")
	}

	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		return errors.Wrap(err, "failed to enable firmware write protect")
	}

	testing.ContextLog(ctx, "Waiting so write protect can enable")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 1s")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	testing.ContextLog(ctx, "Rebooting DUT")
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		return errors.Wrap(err, "failed to perform mode aware reboot")
	}

	if data.wpDecoupled {
		testing.ContextLog(ctx, "Enabling software write protect")
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-enable").Output(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to enable wp")
		}
	}

	// Test writing data to new or existing tag.
	data.writeData = data.testData1
	if err := writeTagToEeprom(ctx, h, d, data); err == nil {
		return errors.Wrap(err, "expected write to failed with wp enabled, but succeeded instead")
	}

	return nil
}

func checkEepromWithoutWP(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	testing.ContextLog(ctx, "Enabling write protect")
	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		return errors.Wrap(err, "failed to disable firmware write protect")
	}

	if err := h.Servo.RunECCommand(ctx, "flashwp false"); err != nil {
		return errors.Wrap(err, "failed to disable flashwp")
	}

	testing.ContextLog(ctx, "Waiting so write protect can disable")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 1s")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	testing.ContextLog(ctx, "Rebooting DUT")
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		return errors.Wrap(err, "failed to perform mode aware reboot")
	}

	if data.wpDecoupled {
		testing.ContextLog(ctx, "Enabling software write protect")
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-disable").Output(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to disable wp")
		}
	}

	// Remove tag if already exists to test creating new tag.
	testing.ContextLogf(ctx, "Checking if tag %q already exists", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, d, data); err == nil {
		testing.ContextLogf(ctx, "tag %q already exists in CBI, removing it", data.cbiTag)
		if err := removeTagFromEeprom(ctx, d, data); err != nil {
			return errors.Wrap(err, "failed to remove tag from cbi")
		}
	}

	// Test writing data to new tag.
	data.writeData = data.testData1
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %q", data.writeData, data.cbiTag)
	if err := writeTagToEeprom(ctx, h, d, data); err != nil {
		return errors.Wrap(err, "failed to write data to cbi")
	}

	testing.ContextLog(ctx, "Sleeping for 5 seconds to let writes complete")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 2 seconds")
	}

	// Data should have written correctly to tag.
	testing.ContextLog(ctx, "Attempting to read data at tag ", data.cbiTag)
	if out, err := readTagFromEeprom(ctx, d, data); err != nil {
		return errors.Wrapf(err, "failed to read data from cbi, got: %v", out)
	} else if out != data.writeData {
		return errors.Wrapf(err, "read data %q, expected to read %q", out, data.writeData)
	}

	// Test changing tag value to new data.
	data.writeData = data.testData2
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %q", data.writeData, data.cbiTag)
	if err := writeTagToEeprom(ctx, h, d, data); err != nil {
		return errors.Wrap(err, "failed to write data to cbi")
	}

	testing.ContextLog(ctx, "Sleeping for 5 seconds to let writes complete")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 2 seconds")
	}

	// Tag value should have been correctly overwritten.
	testing.ContextLog(ctx, "Attempting to read data at tag ", data.cbiTag)
	if out, err := readTagFromEeprom(ctx, d, data); err != nil {
		return errors.Wrap(err, "failed to read data from cbi")
	} else if out != data.writeData {
		return errors.Wrapf(err, "Read data %q, expected to read %q", out, data.writeData)
	}

	// Remove test tag from CBI.
	testing.ContextLog(ctx, "Attempting to remove tag ", data.cbiTag)
	if err := removeTagFromEeprom(ctx, d, data); err != nil {
		return errors.Wrap(err, "failed to remove tag from cbi")
	}

	testing.ContextLog(ctx, "Sleeping for 5 seconds to let tag be removed")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 2 seconds")
	}

	// Verify test tag was successfully removed.
	testing.ContextLogf(ctx, "Verifying tag %q was removed ", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, d, data); err == nil {
		return errors.Wrapf(err, "expected tag %q to be removed from CBI", data.cbiTag)
	}

	return nil
}

func readTagFromEeprom(ctx context.Context, d *dut.DUT, data *eepromData) (string, error) {
	testing.ContextLog(ctx, "Using ectool cbi get to try to read tag (upto 3 retries) ", data.cbiTag)
	strMatch := ""
	err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBIGet, data.cbiTag)
		if err != nil {
			return errors.Wrapf(err, "failed to read tag %q from cbi, got output: %v", data.cbiTag, out)
		}

		cbiGetRegexp := regexp.MustCompile(`As uint:\s*(\S+)\s*\((\S+)\)\sAs binary:\s*(\S+)\s`)
		match := cbiGetRegexp.FindStringSubmatch(out)
		if match == nil || len(match) == 0 {
			return errors.Errorf("cbi read output didn't match expected format, got: %q", out)
		}
		strMatch = match[2]
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		return "", errors.Wrap(err, "failed to read cbi")
	}

	testing.ContextLogf(ctx, "Read data from tag %q: %s", data.cbiTag, strMatch)
	return strMatch, nil
}

func writeTagToEeprom(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %q", data.writeData, data.cbiTag)
	writeArgs := []string{data.cbiTag, data.writeData, strconv.Itoa(data.writeSize)}
	out, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBISet, writeArgs...)
	if err != nil {
		return errors.Wrapf(err, "failed to write data %q to cbi tag %q, got output: %v", data.writeData, data.cbiTag, out)
	}
	return nil
}

func removeTagFromEeprom(ctx context.Context, d *dut.DUT, data *eepromData) error {
	testing.ContextLog(ctx, "Using ectool cbi remove to erase tag ", data.cbiTag)
	out, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBIRemove, data.cbiTag)
	if err != nil {
		return errors.Wrapf(err, "failed to remove tag %q from cbi, got output: %v", data.cbiTag, out)
	}
	return nil
}
