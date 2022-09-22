// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iioservice provides iioservice query util functions for health tast.
package iioservice

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// SensorAttributes stores sensor attributes from iioservice_query.
type SensorAttributes struct {
	Name     *string
	DeviceID int32
	Type     string
	Location string
}

// sensorType coverts raw value to value of sensor type enum in Healthd.
// Unsupported sensor type is acceptable now and return "". If we want to cover
// all sensor types in the future, we should raise error here.
func sensorType(rawType string) string {
	switch rawType {
	case "ACCEL":
		return "Accel"
	case "LIGHT":
		return "Light"
	case "ANGLVEL":
		return "Gyro"
	case "ANGL":
		return "Angle"
	case "GRAVITY":
		return "Gravity"
	default:
		return ""
	}
}

// sensorLocation coverts raw value to value of sensor location enum in Healthd.
func sensorLocation(rawLocation string) string {
	if rawLocation == "base" {
		return "Base"
	} else if rawLocation == "lid" {
		return "Lid"
	} else if rawLocation == "camera" {
		return "Camera"
	} else {
		return "Unknown"
	}
}

var queryLogRegexp = regexp.MustCompile(`.* INFO iioservice_query: .* GetAttributesCallback\(\): ([a-zA-Z ]+): ([a-zA-Z0-9\-]*)`)

// parseQueryLogs parses the logs of iioservice_query for each single sensor.
// Unsupported sensor types will be skipped and return nil.
func parseQueryLogs(logs []string) (*SensorAttributes, error) {
	ret := SensorAttributes{}
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

// For mocking.
var iioserviceQueryCmd = func(ctx context.Context) ([]byte, []byte, error) {
	return testexec.CommandContext(ctx, "iioservice_query", "--device_type=0",
		"--attributes=name location").SeparatedOutput(testexec.DumpLogOnError)
}

// ExpectedSensorAttributes get expected SensorAttributes via iioservice_query.
func ExpectedSensorAttributes(ctx context.Context) ([]SensorAttributes, error) {
	_, bStderr, err := iioserviceQueryCmd(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run iioservice_query command")
	}
	var attrLogs []string
	for _, line := range strings.Split(strings.TrimSpace(string(bStderr)), "\n") {
		if queryLogRegexp.MatchString(line) {
			attrLogs = append(attrLogs, line)
		}
	}

	// Each line is log of iioservice_query. For each sensor, there are four logs,
	// ordered by device ID, type, name, and location.
	const numAttribues = 4
	if len(attrLogs)%numAttribues != 0 {
		return nil, errors.Wrap(err, "failed to parse iioservice_query output")
	}

	var ret []SensorAttributes
	for i := 0; i < len(attrLogs); i += numAttribues {
		attr, err := parseQueryLogs(attrLogs[i : i+numAttribues])
		if err != nil {
			return nil, err
		}
		if attr != nil {
			ret = append(ret, *attr)
		}
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].DeviceID < ret[j].DeviceID })
	return ret, nil
}
