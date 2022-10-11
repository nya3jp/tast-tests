// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ECCbiEeprom,
		Desc:     "Test that ectool can be used to read/write to cbi, and setting write protect prevents writing",
		Contacts: []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:     []string{"group:firmware", "firmware_unstable"},
		Fixture:  fixture.NormalMode,
		Timeout:  15 * time.Minute,
		// Only run on platforms that include CL crrev/c/1234747 so that CBI can be reversibly written to.
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.ECFeatureCBI(), hwdep.SkipOnModel(
			"jax", // Fizz models
			"kench",
			"sion",
			"bard", // Nami models
			"ekko",
			"syndra",
		)),
	})
}

func ECCbiEeprom(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	testTag1 := "99"
	testTag2 := "98"
	testData1 := "32"
	testData2 := "16"
	writeSize := "2"

	defer func() {
		s.Log("Cleaning up wp status")
		if err := setECWriteProtect(ctx, h, false); err != nil {
			s.Fatal("Failed to disable firmware write protect: ", err)
		}

		s.Logf("Removing tag %q", testTag1)
		if err := removeTagFromEeprom(ctx, h, testTag1); err != nil {
			s.Fatal("Expected remove to succeed: ", err)
		}

		s.Logf("Removing tag %q", testTag2)
		if err := removeTagFromEeprom(ctx, h, testTag2); err != nil {
			s.Fatal("Expected remove to succeed: ", err)
		}

	}()

	// Test writing new tag/overwriting existing tag with WP disabled.

	s.Log("Disabling write protect")
	if err := setECWriteProtect(ctx, h, false); err != nil {
		s.Fatal("Failed to disable write protect: ", err)
	}

	// Test writing data to new tag.
	if err := writeTagToEeprom(ctx, h, testTag1, testData1, writeSize); err != nil {
		s.Fatal("Expected write to succeed: ", err)
	}

	if out, err := readTagFromEeprom(ctx, h, testTag1); err != nil {
		s.Fatal("Expected read to succeed: ", err)
	} else if out != testData1 {
		s.Fatalf("Read data different than written data, expected %q got %q: %v", testData1, out, err)
	}

	// Test overwriting data to existing tag.
	if err := writeTagToEeprom(ctx, h, testTag1, testData2, writeSize); err != nil {
		s.Fatal("Expected write to succeed: ", err)
	}

	if out, err := readTagFromEeprom(ctx, h, testTag1); err != nil {
		s.Fatal("Expected read to succeed: ", err)
	} else if out != testData2 {
		s.Fatalf("Read data different than written data, expected %q got %q: %v", testData2, out, err)
	}

	// Test writing new tag/overwriting existing tag with WP enabled.

	s.Log("Enabling write protect")
	if err := setECWriteProtect(ctx, h, true); err != nil {
		s.Fatal("Failed to enable write protect: ", err)
	}

	// Test writing data to new tag with WP enabled.
	if err := writeTagToEeprom(ctx, h, testTag2, testData1, writeSize); err == nil {
		s.Fatal("Expected write to fail: ", err)
	}

	if out, err := readTagFromEeprom(ctx, h, testTag2); err == nil {
		s.Fatal("Expected read to fail since tag shouldn't exist: ", err)
	} else if out == testData1 {
		s.Fatalf("Read data matched written data, read/write should have failed, got %q: %v", out, err)
	}

	// Test writing data to existing tag with WP enabled.
	if err := writeTagToEeprom(ctx, h, testTag1, testData1, writeSize); err == nil {
		s.Fatal("Expected overwrite to fail: ", err)
	}

	s.Logf("Reading from tag %q", testTag1)
	if out, err := readTagFromEeprom(ctx, h, testTag1); err != nil {
		s.Fatal("Expected read to succeed: ", err)
	} else if out == testData1 {
		s.Fatalf("Write should have failed, expected %q, got %q: %v", testData2, out, err)
	}
}

func setECWriteProtect(ctx context.Context, h *firmware.Helper, enable bool) error {
	enableStr := "enable"
	if !enable {
		enableStr = "disable"

		testing.ContextLog(ctx, "Setting fwwpstate to off")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			return errors.Wrap(err, "failed to set fwwpstate to off")
		}
	}

	testing.ContextLogf(ctx, "Setting ec write protect to %q with ec console", enableStr)
	if err := h.Servo.RunECCommand(ctx, fmt.Sprintf("flashwp %t", enable)); err != nil {
		return errors.Wrap(err, "failed to enable flashwp")
	}

	if enable {
		testing.ContextLog(ctx, "Setting fwwpstate to on")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
			return errors.Wrap(err, "failed to set fwwpstate to on")
		}
	}

	testing.ContextLog(ctx, "Rebooting DUT")
	if err := h.DUT.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}
	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to reconnect")
	}

	return nil
}

func writeTagToEeprom(ctx context.Context, h *firmware.Helper, tag, data, size string) error {
	testing.ContextLogf(ctx, "Attempting to write data %q to tag %q", data, tag)
	writeArgs := []string{tag, data, size}
	out, err := firmware.NewECTool(h.DUT, firmware.ECToolNameMain).CBI(ctx, firmware.CBISet, writeArgs...)
	if err != nil {
		return errors.Wrapf(err, "failed to write data %q to cbi tag %q, got output: %v", data, tag, out)
	}
	return nil
}

func readTagFromEeprom(ctx context.Context, h *firmware.Helper, tag string) (string, error) {
	testing.ContextLog(ctx, "Attempting to read data from tag ", tag)
	out, err := firmware.NewECTool(h.DUT, firmware.ECToolNameMain).CBI(ctx, firmware.CBIGet, tag)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read tag %q from cbi, got output: %v", tag, out)
	}
	cbiGetRegexp := regexp.MustCompile(`As uint:\s*(\S+)\s*\(\S+\)`)
	match := cbiGetRegexp.FindStringSubmatch(out)
	if match == nil || len(match) < 2 {
		return "", errors.Errorf("cbi read output didn't match expected format, got: %q", out)
	}
	strMatch := match[1]
	return strMatch, nil
}

func removeTagFromEeprom(ctx context.Context, h *firmware.Helper, tag string) error {
	testing.ContextLog(ctx, "Attempting to remove data from tag ", tag)
	out, err := firmware.NewECTool(h.DUT, firmware.ECToolNameMain).CBI(ctx, firmware.CBIRemove, tag)
	if err != nil {
		return errors.Wrapf(err, "failed to remove tag %q from cbi, got output: %v", tag, out)
	}
	return nil
}
