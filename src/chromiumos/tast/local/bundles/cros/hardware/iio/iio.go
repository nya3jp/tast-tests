// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
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

// SensorReading is one reading from a sensor.
type SensorReading struct {
	// Data contains all values read from the sensor.
	// Its length depends on the type of sensor being used.
	Data []float64
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

// readingNames is a map from the type of sensor to the sensor specific part of the
// sysfs filename for reading raw sensor values. For example the x axis can be read
// from in_accel_x_raw for an accelerometer and in_anglvel_x_raw for a gyroscope.
var readingNames = map[SensorName]string{
	Accel: "accel",
	Gyro:  "anglvel",
	Mag:   "magn",
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

// Read returns the current readings of the sensor.
func (s *Sensor) Read() (SensorReading, error) {
	var ret SensorReading
	sensorPath := path.Join(basePath, iioBasePath, s.Path)
	rName, ok := readingNames[s.Name]
	if !ok {
		return ret, errors.Errorf("cannot read data from %v", s.Name)
	}

	sc, err := ioutil.ReadFile(path.Join(sensorPath, "scale"))
	if err != nil {
		return ret, errors.Wrapf(err, "cannot read %v scale", s.Name)
	}

	scale, err := strconv.ParseFloat(strings.TrimSpace(string(sc)), 64)
	if err != nil {
		return ret, errors.Wrapf(err, "invalid scale %q", sc)
	}

	rawReading := func(axis string) (float64, error) {
		r, err := ioutil.ReadFile(path.Join(sensorPath,
			fmt.Sprintf("in_%s_%s_raw", rName, axis)))
		if err != nil {
			return 0, err
		}

		return strconv.ParseFloat(strings.TrimSpace(string(r)), 64)
	}

	ret.Data = make([]float64, 3)
	for axis, prop := range map[string]*float64{
		"x": &ret.Data[0],
		"y": &ret.Data[1],
		"z": &ret.Data[2],
	} {
		reading, err := rawReading(axis)
		if err != nil {
			return ret, errors.Wrapf(err, "error reading from sensor %v", s.Name)
		}

		*prop = reading * scale
	}

	return ret, nil
}
