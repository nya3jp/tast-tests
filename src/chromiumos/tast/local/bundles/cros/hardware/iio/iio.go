// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"io/ioutil"
	"path"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

// SensorName is the kind of sensor which is reported by the EC and exposed by
// the kernel in /sys/bus/iio/devices/iio:device*/name. The name is in the form
// cros-ec-*.
type SensorName string

// SensorLocation is the location of the sensor in the DUT which is reported by
// the EC and exposed by the kernel in /sys/bus/iio/devices/iio:device*/location.
type SensorLocation string

// Sensor represents one sensor on the DUT.
type Sensor struct {
	Name     SensorName
	Location SensorLocation
	Path     string
}

const (
	// Accel is an accelerometer sensor.
	Accel SensorName = "cros-ec-accel"
	// Gyro is a gyroscope sensor.
	Gyro SensorName = "cros-ec-gyro"
	// Mag is a magnetometer sensor.
	Mag SensorName = "cros-ec-mag"
	// Ring is a special sensor for ChromeOS that produces a stream of data from
	// all sensors on the DUT.
	Ring SensorName = "cros-ec-ring"
)

const (
	// Base means that the sensor is located in the base of the DUT.
	Base SensorLocation = "base"
	// Lid means that the sensor is located in the lid of the DUT.
	Lid SensorLocation = "lid"
	// None means that the sensor location is not known or not applicable.
	None SensorLocation = "none"
)

var sensorNames = map[SensorName]struct{}{
	Accel: {},
	Gyro:  {},
	Mag:   {},
	Ring:  {},
}

var sensorLocations = map[SensorLocation]struct{}{
	Base: {},
	Lid:  {},
}

const iioBasePath = "/sys/bus/iio/devices"

var basePath = ""

// GetSensors enumerates sensors that are exposed by Cros EC as iio devices.
func GetSensors() ([]Sensor, error) {
	var ret []Sensor

	iioPath := path.Join(basePath, iioBasePath)

	files, err := ioutil.ReadDir(iioPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		sensor, err := parseSensor(file.Name(), iioPath)
		if err == nil {
			ret = append(ret, sensor)
		}
	}

	return ret, nil
}

// parseSensor reads the sysfs directory at iioPath/devName and returns a
// Sensor if it is a valid EC sensor.
func parseSensor(devName, iioPath string) (Sensor, error) {
	var sensor Sensor
	var location SensorLocation
	var name SensorName

	re := regexp.MustCompile(`^iio:device[0-9]+$`)
	if !re.MatchString(devName) {
		return sensor, errors.New("not a sensor")
	}

	devPath := path.Join(iioPath, devName)
	rawName, err := ioutil.ReadFile(path.Join(devPath, "name"))
	if err != nil {
		return sensor, errors.Wrap(err, "sensor has no name")
	}

	name = SensorName(strings.TrimSpace(string(rawName)))
	if _, ok := sensorNames[name]; !ok {
		return sensor, errors.Errorf("unknown sensor type %q", name)
	}

	loc, err := ioutil.ReadFile(path.Join(devPath, "location"))
	if err == nil {
		location = SensorLocation(strings.TrimSpace(string(loc)))

		if _, ok := sensorLocations[location]; !ok {
			return sensor, errors.Errorf("unknown sensor loc %q", loc)
		}
	} else {
		location = None
	}

	return Sensor{name, location, devName}, nil
}
