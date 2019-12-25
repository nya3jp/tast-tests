// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	Name          SensorName
	Location      SensorLocation
	Path          string
	ID            uint
	Scale         float64
	MinFrequency  int
	MaxFrequency  int
	OldSysfsStyle bool
}

// SensorReading is one reading from a sensor.
type SensorReading struct {
	// Data contains all values read from the sensor.
	// Its length depends on the type of sensor being used.
	Data  []float64
	ID    uint
	Flags uint8
	// Timestamp is the duration from the boot time of the DUT to the time the
	// reading was taken
	Timestamp time.Duration
}

const (
	// Accel is an accelerometer sensor.
	Accel SensorName = "cros-ec-accel"
	// Gyro is a gyroscope sensor.
	Gyro SensorName = "cros-ec-gyro"
	// Mag is a magnetometer sensor.
	Mag SensorName = "cros-ec-mag"
	// Light is a light or proximity sensor.
	Light SensorName = "cros-ec-light"
	// Sync is a camera-counting sensor.
	Sync SensorName = "cros-ec-sync"
	// Baro is a magnetometer.
	Baro SensorName = "cros-ec-baro"
	// Ring is a special sensor for ChromeOS that produces a stream of data from
	// all sensors on the DUT.
	Ring SensorName = "cros-ec-ring"
)

const (
	// Base means that the sensor is located in the base of the DUT.
	Base SensorLocation = "base"
	// Lid means that the sensor is located in the lid of the DUT.
	Lid SensorLocation = "lid"
	// Camera means that the sensor is located in the camera of the DUT.
	Camera SensorLocation = "camera"
	// None means that the sensor location is not known or not applicable.
	None SensorLocation = "none"
)

// cros ec data flags from ec_commands.h
const (
	flushFlag      = 0x01
	timestampFlag  = 0x02
	wakeupFlag     = 0x04
	tabletModeFlag = 0x08
	odrFlag        = 0x10
)

var sensorNames = map[SensorName]struct{}{
	Accel: {},
	Baro:  {},
	Gyro:  {},
	Light: {},
	Mag:   {},
	Ring:  {},
	Sync:  {},
}

var sensorLocations = map[SensorLocation]struct{}{
	Base:   {},
	Lid:    {},
	Camera: {},
}

// readingNames is a map from the type of sensor to the sensor specific part of the
// sysfs filename for reading raw sensor values. For example the x axis can be read
// from in_accel_x_raw for an accelerometer and in_anglvel_x_raw for a gyroscope.
var readingNames = map[SensorName]string{
	Accel: "accel",
	Gyro:  "anglvel",
	Mag:   "magn",
}

const iioBasePath = "sys/bus/iio/devices"

var basePath = "/"

// GetSensors enumerates sensors that are exposed by Cros EC as iio devices.
func GetSensors() ([]*Sensor, error) {
	var ret []*Sensor

	fullpath := filepath.Join(basePath, iioBasePath)

	// Some systems will not have any iio devices; this case should not be an error.
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		return ret, nil
	}

	files, err := ioutil.ReadDir(fullpath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		sensor, err := parseSensor(file.Name())
		if err == nil {
			ret = append(ret, sensor)
		}
	}

	return ret, nil
}

// parseSensor reads the sysfs directory at iioBasePath/devName and returns a
// Sensor if it is a valid EC sensor.
func parseSensor(devName string) (*Sensor, error) {
	var sensor Sensor
	var location SensorLocation
	var name SensorName
	var id, minFreq, maxFreq int
	var scale float64
	var zeroInt, zeroFrac, minInt, minFrac, maxInt, maxFrac int

	re := regexp.MustCompile(`^iio:device[0-9]+$`)
	if !re.MatchString(devName) {
		return nil, errors.New("not a sensor")
	}

	sensor.Path = devName

	rawName, err := sensor.ReadAttr("name")
	if err != nil {
		return nil, errors.Wrap(err, "sensor has no name")
	}

	name = SensorName(rawName)
	if _, ok := sensorNames[name]; !ok {
		return nil, errors.Errorf("unknown sensor type %q", name)
	}

	loc, err := sensor.ReadAttr("location")
	if err == nil {
		location = SensorLocation(loc)

		if _, ok := sensorLocations[location]; !ok {
			return nil, errors.Errorf("unknown sensor loc %q", loc)
		}
	} else {
		location = None
	}

	s, err := sensor.ReadAttr("scale")
	if err == nil {
		scale, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid scale %q", s)
		}
	}

	i, err := sensor.ReadAttr("id")
	if err == nil {
		id, err = strconv.Atoi(i)
		if err != nil {
			return nil, errors.Wrapf(err, "bad sensor id %q", i)
		}

		if id < 0 {
			return nil, errors.Errorf("invalid sensor id %v", id)
		}
	}

	_, err = sensor.ReadAttr("frequency")
	sensor.OldSysfsStyle = err == nil

	if sensor.OldSysfsStyle {
		f, err := sensor.ReadAttr("min_frequency")
		if err == nil {
			minFreq, err = strconv.Atoi(f)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid min frequency %q", f)
			}
		}

		f, err = sensor.ReadAttr("max_frequency")
		if err == nil {
			maxFreq, err = strconv.Atoi(f)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid max frequency %q", f)
			}
		}
	} else {
		f, err := sensor.ReadAttr("sampling_frequency_available")
		if err == nil {
			_, err = fmt.Sscanf(f, "%d.%06d %d.%06d %d.%06d",
				&zeroInt, &zeroFrac, &minInt, &minFrac, &maxInt, &maxFrac)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid frequency range %q", f)
			}
			if zeroInt != 0 || zeroFrac != 0 {
				return nil, errors.Wrapf(err, "frequency range must start with 0 %q", f)
			}
			// In this code, frequency unit is mHz. iio now reports frequency in Hz with
			// 6 digits of precision.
			// So 12.5Hz will be printed 12.500000.
			// Int will be 12, Frac 500000.
			minFreq = minInt*1000 + minFrac/1000
			maxFreq = maxInt*1000 + maxFrac/1000
		}
	}

	sensor.Name = name
	sensor.Location = location
	sensor.ID = uint(id)
	sensor.Scale = scale
	sensor.MinFrequency = minFreq
	sensor.MaxFrequency = maxFreq

	return &sensor, nil
}

// Read returns the current readings of the sensor.
func (s *Sensor) Read() (*SensorReading, error) {
	var ret SensorReading
	rName, ok := readingNames[s.Name]
	if !ok {
		return nil, errors.Errorf("cannot read data from %v", s.Name)
	}

	rawReading := func(axis string) (float64, error) {
		r, err := s.ReadAttr(fmt.Sprintf("in_%s_%s_raw", rName, axis))
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
			return nil, errors.Wrapf(err, "error reading from sensor %v", s.Name)
		}

		*prop = reading * s.Scale
	}

	return &ret, nil
}

// WriteAttr writes value to the sensor's attr file.
func (s *Sensor) WriteAttr(attr, value string) error {
	err := ioutil.WriteFile(filepath.Join(basePath, iioBasePath, s.Path, attr),
		[]byte(value), os.ModePerm)

	if err != nil {
		return errors.Wrapf(err, "error writing attribute %q of %v", attr, s.Path)
	}

	return nil
}

// ReadAttr reads the sensor's attr file and returns the value.
func (s *Sensor) ReadAttr(attr string) (string, error) {
	a, err := ioutil.ReadFile(filepath.Join(basePath, iioBasePath, s.Path, attr))
	if err != nil {
		return "", errors.Wrapf(err, "error reading attribute %q of %v", attr, s.Path)
	}
	return strings.TrimSpace(string(a)), nil
}
