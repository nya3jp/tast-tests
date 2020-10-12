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
	// Find activity sensor.
	var sensor *iio.Sensor
	for _, sn := range sensors {
		if sn.Name == iio.Activity {
			sensor = sn
		}
	}
	if sensor == nil {
		s.Log("No activity sensor is found")
		return
	}
	id := strconv.Itoa(int(sensor.ID))

	// Query original spoofing state
	// ectool motionsense spoof ActivitySenserID activity BodyDetectionID
	output, err := testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", id, "activity", "4").Output()
	if err != nil {
		s.Fatal("ectool failed to query spoofing activity: ", err)
	}
	originalSpoofEnable := 0
	originalSpoofState := "0"
	if strings.Contains(string(output), "enabled") {
		originalSpoofEnable = 1
		originalSpoofState, err = sensor.ReadAttr("in_proximity_raw")
		if err != nil {
			s.Fatal("Failed to read originalSpoofState: ", err)
		}
	} else {
		// Enable spoofing activity by ectool command first to prevent unexpected proximity event.
		// ectool motionsense spoof ActivitySenserID activity BodyDetectionID Enable
		_, err = testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", id, "activity", "4", "1").Output()
		if err != nil {
			s.Fatal("ectool failed to enable spoofing activity: ", err)
		}
	}

	// Start to listen on powerd log.
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

	// Test if on-body detection can detect on-body and off-body.
	s.Log("[Testing on-body detection]: ")
	if err := testOnBodyDetection(ctx, sensor, reader, 1); err != nil {
		s.Fatal("Failed to switch to on-body: ", err)
	}
	if err := testOnBodyDetection(ctx, sensor, reader, 0); err != nil {
		s.Fatal("Failed to switch to off-body: ", err)
	}

	// Recover spoofing state.
	if originalSpoofEnable == 0 {
		// ectool motionsense spoof ActivitySenserID activity BodyDetectionID Disable
		_, err = testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", id, "activity", "4", "0").Output()
		if err != nil {
			s.Fatal("ectool failed to disable spoofing activity: ", err)
		}
	} else {
		// ectool motionsense spoof ActivitySenserID activity BodyDetectionID Enable originalSpoofState
		_, err = testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", id, "activity", "4", "1", originalSpoofState).Output()
		if err != nil {
			s.Fatal("ectool failed to recover spoofing activity state: ", err)
		}
	}
}

func testOnBodyDetection(ctx context.Context, sensor *iio.Sensor, scanner *bufio.Scanner, state int) error {
	id := strconv.Itoa(int(sensor.ID))
	// Spoof the activity to given state.
	// ectool motionsense spoof ActivitySenserID activity BodyDetectionID Enable ActivityState
	_, err := testexec.CommandContext(ctx, "ectool", "motionsense", "spoof", id, "activity", "4", "1", strconv.Itoa(state)).Output()
	if err != nil {
		return errors.Wrap(err, "ectool failed to spoof activity: ")
	}

	// Read the proximity state from the sensor's attribute, in order to
	// check if the kernel receives proximity event from EC.
	rawProximity, err := sensor.ReadAttr("in_proximity_raw")
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

		// Check if the content contains the proximity log.
		// -1 if not found, 0 for off-body and 1 for on-body.
		proximity := -1
		if strings.HasSuffix(content, "User proximity: Near") {
			proximity = 1
		} else if strings.HasSuffix(content, "User proximity: Far") {
			proximity = 0
		}

		if proximity != -1 {
			if proximity != state {
				return errors.Errorf("powerd proximity state is %d, expected %d", proximity, state)
			}
			// Succeeded to find the correct proximity log
			return nil
		}
	}
	// Failed to find any proximity log
	return errors.New("not found any proximity event in the log")
}
