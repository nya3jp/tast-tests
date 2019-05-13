// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

type (
	// SensorName is the kind of sensor
	SensorName string
	// SensorLocation is the location of the sensor in the dut
	SensorLocation string
	// Sensor represents one sensor on the dut
	Sensor struct {
		Name     SensorName
		Location SensorLocation
		Path     string
	}
)

const (
	// Accel an accelerometer sensor
	Accel SensorName = "cros-ec-accel"
	// Gyro a gyroscope sensor
	Gyro SensorName = "cros-ec-gyro"
	// Mag a magnetometer sensor
	Mag SensorName = "cros-ec-mag"
	// Ring a special sensor for ChromeOS that produces a stream of data from all
	// sensors in the system
	Ring SensorName = "cros-ec-ring"
)

const (
	// Base the sensor is located in the base of the dut
	Base SensorLocation = "base"
	// Lid the sensor is located in the lid of the dut
	Lid SensorLocation = "lid"
	// None the sensor location is not known or not applicable
	None SensorLocation = "none"
)

var sensorTypeMap = map[string]SensorName{
	string(Accel): Accel,
	string(Gyro):  Gyro,
	string(Mag):   Mag,
	string(Ring):  Ring,
}

var sensorLocationMap = map[string]SensorLocation{
	string(Base): Base,
	string(Lid):  Lid,
}

const (
	iioBasePath = "/sys/bus/iio/devices"
)

var basePath = ""

// GetSensors get a list of sensors on the dut
func GetSensors() ([]Sensor, error) {
	var ret []Sensor

	iioPath := path.Join(basePath, iioBasePath)

	parseSensor := func(file os.FileInfo) (Sensor, error) {
		var sensor Sensor

		re := regexp.MustCompile(`^iio:device[0-9]+$`)

		if !re.MatchString(file.Name()) {
			return sensor, errors.New("Not a sensor")
		}

		devPath := path.Join(iioPath, file.Name())

		name, err := ioutil.ReadFile(path.Join(devPath, "name"))

		if err != nil {
			return sensor, errors.New("Sensor has no name")
		}

		sensorName, ok := sensorTypeMap[strings.TrimSpace(string(name))]

		if !ok {
			return sensor, errors.Errorf("Unknown sensor type %q", string(name))
		}

		loc, err := ioutil.ReadFile(path.Join(devPath, "location"))
		location := None

		if err == nil {
			location, ok = sensorLocationMap[strings.TrimSpace(string(loc))]

			if !ok {
				return sensor, errors.Errorf("Unknown sensor loc %q", string(loc))
			}
		}

		return Sensor{sensorName, location, file.Name()}, nil
	}

	files, err := ioutil.ReadDir(iioPath)

	if err != nil {
		return nil, err
	}

	for _, file := range files {
		sensor, err := parseSensor(file)

		if err == nil {
			ret = append(ret, sensor)
		}
	}

	return ret, nil
}
