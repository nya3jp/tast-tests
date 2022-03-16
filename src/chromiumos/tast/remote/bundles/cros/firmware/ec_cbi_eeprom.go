// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	oldData     string
	writeData   string
	newData     string
	writeSize   int
	cbiTag      int
	pageSize    int
}

func ECCbiEeprom(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	pageSize := 8
	writeSize := (pageSize + 1) / 2 // + 1 in case page size is made an odd num
	testCbiTag := 99

	// Generate random data to test writes on.
	bytes := make([]byte, writeSize)
	if _, err := rand.Read(bytes); err != nil {
		s.Fatal("Failed to generate random number: ", err)
	}
	testData1 := fmt.Sprintf("0x%s", hex.EncodeToString(bytes))
	if _, err := rand.Read(bytes); err != nil {
		s.Fatal("Failed to generate random number: ", err)
	}
	testData2 := fmt.Sprintf("0x%s", hex.EncodeToString(bytes))

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
		cbiTag: testCbiTag, pageSize: pageSize,
		writeSize: writeSize, testData1: testData1, testData2: testData2,
	}

	s.Log("Use ectool gpioget to check if cbi wp is decoupled")
	out, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).GPIOGet(ctx, firmware.ECCbiWp)
	if err != nil {
		s.Log("CBI WP is not decoupled from EC WP")
	} else {
		s.Log("CBI WP is decoupled from EC WP: ", out)
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

	}()

	s.Log("Test EEPROM is not writable with WP enabled")
	if err := checkEepromWithWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp enabled")
	}

	s.Log("Test EEPROM is writable with WP disabled")
	if err := checkEepromWithoutWP(ctx, h, s.DUT(), data); err != nil {
		s.Fatal("EEPROM read/write test failed with wp disabled")
	}

}

func checkEepromWithWP(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	// Remove tag if already exists before enabling wp to test creating new tag.
	testing.ContextLogf(ctx, "Checking if tag %q already exists", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, h, d, data); err == nil {
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
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %d", data.writeData, data.cbiTag)
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
	testing.ContextLogf(ctx, "Checking if tag %d already exists", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, h, d, data); err == nil {
		testing.ContextLogf(ctx, "tag %d already exists in CBI, removing it", data.cbiTag)
		if err := removeTagFromEeprom(ctx, d, data); err != nil {
			return errors.Wrap(err, "failed to remove tag from cbi")
		}
	}

	// Test writing data to new tag.
	data.writeData = data.testData1
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %d", data.writeData, data.cbiTag)
	if err := writeTagToEeprom(ctx, h, d, data); err != nil {
		return errors.Wrap(err, "failed to write data to cbi")
	}

	testing.ContextLog(ctx, "Sleeping for 2 seconds to let writes flush")
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 2 seconds")
	}

	// Data should have written correctly to tag.
	testing.ContextLog(ctx, "Attempting to read data at tag ", data.cbiTag)
	if out, err := readTagFromEeprom(ctx, h, d, data); err != nil {
		return errors.Wrapf(err, "failed to read data from cbi, got: %v", out)
	} else if out != data.writeData {
		return errors.Wrapf(err, "read data %q, expected to read %q", out, data.writeData)
	}

	// Test changing tag value to new data.
	data.writeData = data.testData2
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %d", data.writeData, data.cbiTag)
	if err := writeTagToEeprom(ctx, h, d, data); err != nil {
		return errors.Wrap(err, "failed to write data to cbi")
	}

	testing.ContextLog(ctx, "Sleeping for 2 seconds to let writes flush")
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 2 seconds")
	}

	// Tag value should have been correctly overwritten.
	testing.ContextLog(ctx, "Attempting to read data at tag ", data.cbiTag)
	if out, err := readTagFromEeprom(ctx, h, d, data); err != nil {
		return errors.Wrap(err, "failed to read data from cbi")
	} else if out != data.writeData {
		return errors.Wrapf(err, "Read data %q, expected to read %q", out, data.writeData)
	}

	// Remove test tag from CBI.
	testing.ContextLog(ctx, "Attempting to remove tag ", data.cbiTag)
	if err := removeTagFromEeprom(ctx, d, data); err != nil {
		return errors.Wrap(err, "failed to remove tag from cbi")
	}

	// Verify test tag was successfully removed.
	testing.ContextLogf(ctx, "Verifying tag %q was removed ", data.cbiTag)
	if _, err := readTagFromEeprom(ctx, h, d, data); err == nil {
		return errors.Wrapf(err, "expected tag %q to be removed from CBI", data.cbiTag)
	}

	return nil
}

func readTagFromEeprom(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) (string, error) {
	testing.ContextLog(ctx, "Using ectool cbi get to read tag ", data.cbiTag)
	out, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBIGet, strconv.Itoa(data.cbiTag), "01")
	if err != nil {
		return "", errors.Wrapf(err, "failed to read tag %d from cbi", data.cbiTag)
	}
	cbiGetRegexp := regexp.MustCompile(`As uint:\s*(\S+)\s*\((\S+)\)\sAs binary:\s*(\S+)\s`)
	match := cbiGetRegexp.FindStringSubmatch(out)
	if match == nil || len(match) == 0 {
		return "", errors.Errorf("cbi read output didn't match expected format, got: %q", out)
	}
	testing.ContextLogf(ctx, "Read data from tag %d: %s", data.cbiTag, match[2])
	return match[2], nil
}

func writeTagToEeprom(ctx context.Context, h *firmware.Helper, d *dut.DUT, data *eepromData) error {
	writeArgs := []string{
		"cbi", "set", strconv.Itoa(data.cbiTag), data.testData1,
		strconv.Itoa(data.writeSize),
	}

	testing.ContextLogf(ctx, "Using ectool cbi set to write %q to tag %d", data.writeData, data.cbiTag)
	// out, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBISet, writeArgs...)
	out, err := h.DUT.Conn().CommandContext(ctx, "ectool", writeArgs...).CombinedOutput(ssh.DumpLogOnError)
	testing.ContextLogf(ctx, "The write op output was: %v; %v", string(out), err)
	if err != nil {
		return errors.Wrapf(err, "failed to write data %q to cbi tag %d", data.writeData, data.cbiTag)
	}
	return nil
}

func removeTagFromEeprom(ctx context.Context, d *dut.DUT, data *eepromData) error {
	testing.ContextLog(ctx, "Using ectool cbi remove to erase tag ", data.cbiTag)
	_, err := firmware.NewECTool(d, firmware.ECToolNameMain).CBI(ctx, firmware.CBIRemove, strconv.Itoa(data.cbiTag))
	if err != nil {
		return errors.Wrapf(err, "failed to remove tag %q from cbi", data.cbiTag)
	}
	return nil
}
