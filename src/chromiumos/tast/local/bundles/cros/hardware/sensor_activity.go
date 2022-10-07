// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorActivity,
		Desc: "Tests that activity sensors can be read and give proximity event",
		Contacts: []string{
			"gwendal@chromium.com",   // ChromeOS sensors point of contact
			"chingkang@chromium.org", // Test author
			"chromeos-sensors-eng@google.com",
		},
		Attr:         []string{"group:sensors"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.Model("coachz")),
		}},
	})
}

func SensorActivity(ctx context.Context, s *testing.State) {
	sensors, err := iio.GetSensors(ctx)
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
	activities, err := listActivities(ctx, s)
	if err != nil {
		s.Fatal("Error listing activities: ", err)
	}
	for _, x := range activities {
		switch x {
		case iio.OnBodyDetectionID:
			s.Log("Testing on-body detection:")
			testOnBodyDetection(ctx, s, sensor)
		default:
			s.Logf("Testing activity %d is not implemented", int(x))
		}
	}
}

// testOnBodyDetection tests whether the on-body detection works properly on the DUT.
func testOnBodyDetection(ctx context.Context, s *testing.State, sensor *iio.Sensor) {
	// Query and save the original status of spoofing on-body detection: {spoofEnable, spoofState}
	// spoofEnable: whether the spoofing is enable
	// spoofState: while the spoofing is enabled, what the activity state is.
	output, err := spoofActivity(ctx, sensor, iio.OnBodyDetectionID)
	if err != nil {
		s.Fatal("ectool failed to query spoofEnable of on-body detection: ", err)
	}
	originalSpoofEnable := strings.Contains(string(output), "enabled")
	originalSpoofState := 0
	if originalSpoofEnable {
		originalSpoofState, err = sensor.ReadIntegerAttr("in_proximity_raw")
		if err != nil {
			s.Fatal("Failed to read spoofState: ", err)
		}
	} else {
		// Enable spoofing on-body detection first to prevent unexpected proximity event.
		_, err = spoofActivity(ctx, sensor, iio.OnBodyDetectionID, 1)
		if err != nil {
			s.Fatal("ectool failed to enable spoofing on-body detection: ", err)
		}
	}
	defer func() {
		s.Log("Recover the status of spoofing on-body detection")
		if originalSpoofEnable {
			// Spoof the on-body detection to origin state.
			_, err = spoofActivity(ctx, sensor, iio.OnBodyDetectionID, 1, originalSpoofState)
			if err != nil {
				s.Fatal("ectool failed to recover spoofing on-body detection state: ", err)
			}
		} else {
			// Disable the on-body detection spoofing.
			_, err = spoofActivity(ctx, sensor, iio.OnBodyDetectionID, 0)
			if err != nil {
				s.Fatal("ectool failed to disable spoofing on-body detection: ", err)
			}
		}
	}()

	// Start to listen on powerd log.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(5*time.Second))
	defer cancel()
	cmd := testexec.CommandContext(timeoutCtx, "tail", "-n", "0", "-f", "/var/log/power_manager/powerd.LATEST")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to create stdout pipe of cmd: ", err)
	}
	reader := bufio.NewScanner(stdout)
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start cmd: ", err)
	}

	// Test if on-body detection can detect on-body and off-body.
	if err := testOnBodyDetectionStateChange(ctx, sensor, reader, 1); err != nil {
		s.Fatal("Failed to switch to on-body: ", err)
	}
	if err := testOnBodyDetectionStateChange(ctx, sensor, reader, 0); err != nil {
		s.Fatal("Failed to switch to off-body: ", err)
	}
}

// spoofActivity is a helper function for command line: ectool motionsense spoof sensorID activity activityID [enable/disable] [state]
// If the |args| is empty, it queries the spoofing enable status.
// See https://chromium.googlesource.com/chromiumos/platform/ec/+/refs/heads/main/util/ectool.c for more details.
func spoofActivity(ctx context.Context, sensor *iio.Sensor, act iio.ActivityID, args ...int) (output []byte, err error) {
	id := strconv.Itoa(int(sensor.ID))
	strArgs := []string{"motionsense", "spoof", id, "activity", strconv.Itoa(int(act))}
	for _, arg := range args {
		s := strconv.Itoa(arg)
		strArgs = append(strArgs, s)
	}
	return testexec.CommandContext(ctx, "ectool", strArgs...).Output()
}

// listActivities is a helper function for command line: ectool motionsense list_activities
// It will return the ID of all available activities on the DUT.
// See https://chromium.googlesource.com/chromiumos/platform/ec/+/refs/heads/main/util/ectool.c for more details.
func listActivities(ctx context.Context, s *testing.State) (activities []iio.ActivityID, err error) {
	rawOutput, err := testexec.CommandContext(ctx, "ectool", "motionsense", "list_activities").Output()
	if err != nil {
		return nil, errors.Wrap(err, "ectool failed to list the activities")
	}
	for _, line := range strings.Split(string(rawOutput), "\n") {
		re := regexp.MustCompile("^[0-9]+")
		idData := re.Find([]byte(line))
		if idData != nil {
			id, err := strconv.Atoi(string(idData))
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert activity id")
			}
			activities = append(activities, iio.ActivityID(id))
		}
	}
	return
}

// testOnBodyDetectionStateChange tests whether the DUT can handle the event when on-body detection change status to the given state.
// Note that this function changes the spoofing status of on-body detection.
func testOnBodyDetectionStateChange(ctx context.Context, sensor *iio.Sensor, scanner *bufio.Scanner, state int) error {
	// Spoof the on-body detection to given state.
	_, err := spoofActivity(ctx, sensor, iio.OnBodyDetectionID, 1, state)
	if err != nil {
		return errors.Wrap(err, "ectool failed to spoof on-body detection")
	}

	// Read the proximity state from the sensor's attribute, in order to
	// check if the kernel receives proximity event from EC.
	proximity, err := sensor.ReadIntegerAttr("in_proximity_raw")
	if err != nil {
		return errors.Wrap(err, "failed to read in_proximity_raw")
	}
	if proximity != state {
		return errors.Errorf("in_proximity_raw state is %d, expected %d", proximity, state)
	}

	// Read the proximity state from powerd log, in order to check if the
	// powerd receives and handles the proximity event properly.
	for scanner.Scan() {
		content := scanner.Text()
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
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "error reading powerd log")
	}
	// Failed to find any proximity log
	return errors.New("not found any proximity event in the log")
}
