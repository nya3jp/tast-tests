// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorActivity,
		Desc: "Tests that activity sensors can be read and give proximity event",
		Contacts: []string{
			"gwendal@chromium.com",   // Chrome OS sensors point of contact
			"chingkang@chromium.org", // Test author
			"chromeos-sensors-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func SensorActivity(ctx context.Context, s *testing.State) {
	sensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	// Listen on powerd log.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(5*time.Second))
	defer cancel()
	cmd := testexec.CommandContext(timeoutCtx, "tail", "-n", "0", "-f", "/var/log/power_manager/powerd.LATEST")
	stdout, err := cmd.StdoutPipe()
	reader := bufio.NewScanner(stdout)
	if err != nil {
		s.Fatal("Failed to create stdout pipe of cmd: ", err)
	}
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start cmd: ", err)
	}

	for _, sn := range sensors {
		if sn.Name != iio.Activity {
			continue
		}

		err = testOnBodyDetection(ctx, sn, reader, 1)
		if err != nil {
			s.Fatal("Failed to switch to on_body: ", err)
		}
		err = testOnBodyDetection(ctx, sn, reader, 0)
		if err != nil {
			s.Fatal("Failed to switch to off_body: ", err)
		}

		// Disable spoofing activity by ectool command.
		_, err = testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", strconv.Itoa(int(sn.ID)), "activity", "4", "0").Output()
		if err != nil {
			s.Fatal("ectool failed to disable spoofing activity: ", err)
		}
	}
}

func testOnBodyDetection(ctx context.Context, sn *iio.Sensor, scanner *bufio.Scanner, state int) error {
	ID := strconv.Itoa(int(sn.ID))

	// Spoof the activity to given state.
	_, err := testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", ID, "activity", "4", "1", strconv.Itoa(state)).Output()
	if err != nil {
		return errors.Wrap(err, "ectool failed to spoof activity: ")
	}

	// Read the proximity state from the sensor's attribute, in order to
	// check if the kernel receives proximity event from EC.
	rawProximity, err := sn.ReadAttr("in_proximity_raw")
	if err != nil {
		return errors.Wrap(err, "failed to read in_proximity_raw: ")
	}
	proximity, err := strconv.Atoi(rawProximity)
	if err != nil {
		return errors.Wrapf(err, "rawProximity %s is not integer: ", rawProximity)
	}
	if proximity != state {
		return errors.Errorf("in_proximity_raw state is %d, expected %d", proximity, state)
	}

	// Read the proximity state from powerd log, in order to check if the
	// powerd receives and handles the proximity event properly.
	for scanner.Scan() {
		content := scanner.Text()
		if err := scanner.Err(); err != nil {
			break
		}
		proximity := -1
		if strings.HasSuffix(content, "Proximity: Near") {
			proximity = 1
		} else if strings.HasSuffix(content, "Proximity: Far") {
			proximity = 0
		}

		if proximity != -1 {
			if proximity != state {
				return errors.Errorf("powerd proximity state is %d, expected %d", proximity, state)
			}
			return nil
		}
	}
	return errors.New("not found any proximity event in the log")
}
