// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type sensorInfo struct {
	LidAngle *uint16            `json:"lid_angle"`
	Sensors  []sensorAttributes `json:"sensors"`
}

type sensorAttributes struct {
	Name     *string `json:"name"`
	DeviceID int32   `json:"device_id"`
	Type     string  `json:"type"`
	Location string  `json:"location"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeSensorInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch sensor info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

// rawLidAngle parses the output of ectool and gets the raw value of lid angle.
// The return string should be a number or "unreliable".
func rawLidAngle(ctx context.Context) (string, error) {
	// Check to see if a Google EC exists. If it does, use ectool to get the lid
	// angle that should be reported. Otherwise, return "" if the device does not
	// have a Google EC.
	if _, err := os.Stat("/sys/class/chromeos/cros_ec"); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	bStdout, bStderr, err := testexec.CommandContext(ctx, "ectool", "motionsense", "lid_angle").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		stderr := string(bStderr)
		if strings.Contains(stderr, "INVALID_COMMAND") || strings.Contains(stderr, "INVALID_PARAM") {
			// Some devices do not support lid_angle and return |INVALID_COMMAND| or
			// |INVALID_PARAM|. Check stderr and return "" in these cases.
			return "", nil
		}
		return "", errors.Wrap(err, "failed to run ectool command")
	}

	return strings.ReplaceAll(strings.TrimSpace(string(bStdout)), "Lid angle: ", ""), nil
}

func validateLidAngle(ctx context.Context, info *sensorInfo) error {
	lidAngleRaw, err := rawLidAngle(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get lid angle")
	}

	if lidAngleRaw == "" || lidAngleRaw == "unreliable" {
		if info.LidAngle != nil {
			return errors.New("there is no reliable LidAngle, but cros_healthd report it")
		}
	} else {
		lidAngle, err := strconv.ParseUint(lidAngleRaw, 10, 16)
		if err != nil {
			return err
		}
		if info.LidAngle == nil {
			return errors.Errorf("failed. LidAngle doesn't match: got nil; want %v", lidAngle)
		}
		// The value of lid angle comes from the value of accelerometers on lid and
		// base, which is dynamic without user interaction. We should have the lid
		// angle tolerance.
		const lidAngleTolerance = 1
		if math.Abs(float64(*info.LidAngle)-float64(lidAngle)) > lidAngleTolerance {
			return errors.Errorf("failed. LidAngle doesn't match: got %v; want %v", *info.LidAngle, lidAngle)
		}
	}

	return nil
}

// sensorType covert raw value to value of sensor type enum in Healthd.
// Unsupported sensor type is acceptable now and return "". If we want to cover
// all sensor types in the future, we should raise error here.
func sensorType(rawType string) string {
	if rawType == "ACCEL" {
		return "Accel"
	} else if rawType == "LIGHT" {
		return "Light"
	} else if rawType == "ANGLVEL" {
		return "Gyro"
	} else if rawType == "ANGL" {
		return "Angle"
	} else if rawType == "GRAVITY" {
		return "Gravity"
	} else {
		return ""
	}
}

// sensorLocation covert raw value to value of sensor location enum in Healthd.
func sensorLocation(rawLoaction string) string {
	if rawLoaction == "base" {
		return "Base"
	} else if rawLoaction == "lid" {
		return "Lid"
	} else if rawLoaction == "camera" {
		return "Camera"
	} else {
		return "Unknown"
	}
}

var queryLogRegexp = regexp.MustCompile(`.* INFO iioservice_query: .* GetAttributesCallback\(\): ([a-zA-Z ]+): ([a-zA-Z0-9\-]*)`)

// parseQueryLogs parses the logs of iioservice_query for each single sensor.
// Unsupported sensor types will be skipped and return nil.
func parseQueryLogs(logs []string) (*sensorAttributes, error) {
	ret := sensorAttributes{}
	for _, line := range logs {
		// Format: "... INFO iioservice_query: ... GetAttributesCallback(): ${ATTRIBUTE_NAME}: ${ATTRIBUTE_VALUE}\n"
		queryLogMatch := queryLogRegexp.FindStringSubmatch(line)
		if queryLogMatch == nil || len(queryLogMatch) != 3 {
			return nil, errors.Errorf("failed to parse sensor attributes, log: %s", line)
		}
		attrName, attrValue := queryLogMatch[1], queryLogMatch[2]
		if attrName == "Device id" {
			deviceID, err := strconv.ParseInt(attrValue, 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse sensor device ID, log: %s", line)
			}
			ret.DeviceID = int32(deviceID)
		} else if attrName == "Type" {
			sensorType := sensorType(attrValue)
			// Unsupported sensor type will be skipped.
			if sensorType == "" {
				return nil, nil
			}
			ret.Type = sensorType
		} else if attrName == "name" {
			if attrValue != "" {
				ret.Name = &attrValue
			}
		} else if attrName == "location" {
			ret.Location = sensorLocation(attrValue)
		} else {
			return nil, errors.Errorf("unrecognized attribute field: %s, log: %s", attrName, line)
		}
	}
	return &ret, nil
}

// expectedSensorAttributes get expected sensorAttributes via iioservice_query.
func expectedSensorAttributes(ctx context.Context) (*[]sensorAttributes, error) {
	_, bStderr, err := testexec.CommandContext(ctx, "iioservice_query", "--device_type=0",
		"--attributes=name location").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run iioservice_query command")
	}

	// Each line is log of iioservice_query. For each sensor, there are four logs,
	// ordered by device ID, type, name, and location.
	const numAttribues = 4
	lines := strings.Split(strings.TrimSpace(string(bStderr)), "\n")
	if len(lines)%numAttribues != 0 {
		return nil, errors.Wrap(err, "failed to parse iioservice_query output")
	}

	var ret []sensorAttributes
	for i := 0; i < len(lines); i += numAttribues {
		attr, err := parseQueryLogs(lines[i : i+numAttribues])
		if err != nil {
			return nil, err
		}
		if attr != nil {
			ret = append(ret, *attr)
		}
	}
	return &ret, nil
}

func validateSensorAttributes(ctx context.Context, got *[]sensorAttributes) error {
	expected, err := expectedSensorAttributes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get expected sensor attributes")
	}
	sort.Slice(*expected, func(i, j int) bool {
		return (*expected)[i].DeviceID < (*expected)[j].DeviceID
	})
	sort.Slice(*got, func(i, j int) bool {
		return (*got)[i].DeviceID < (*got)[j].DeviceID
	})
	if diff := cmp.Diff(expected, got); diff != "" {
		return errors.Wrapf(err, "sensor attributes mismatch (-expected + got): %s", diff)
	}
	return nil
}

func ProbeSensorInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySensor}
	var info sensorInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get sensor telemetry info: ", err)
	}

	if err := validateSensorAttributes(ctx, &info.Sensors); err != nil {
		s.Fatal("Failed to validate sensor attributes: ", err)
	}

	if err := validateLidAngle(ctx, &info); err != nil {
		s.Fatal("Failed to validate lid angle: ", err)
	}
}
