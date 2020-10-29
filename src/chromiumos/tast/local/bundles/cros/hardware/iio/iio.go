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

// Device is a object we can read and write attributes from.
type Device struct {
	Path string
}

// SensorName is the kind of sensor which is reported by the EC and exposed by
// the kernel in /sys/bus/iio/devices/iio:device*/name. The name is in the form
// cros-ec-*.
type SensorName string

// SensorLocation is the location of the sensor in the DUT which is reported by
// the EC and exposed by the kernel in /sys/bus/iio/devices/iio:device*/location.
type SensorLocation string

// ActivityID is the ID of the activity which is reported by the EC.
type ActivityID int

// Sensor represents one sensor on the DUT.
type Sensor struct {
	Device
	Name          SensorName
	Location      SensorLocation
	IioID         uint
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
	// Baro is a barometer.
	Baro SensorName = "cros-ec-baro"
	// Ring is a special sensor for ChromeOS that produces a stream of data from
	// all sensors on the DUT.
	Ring SensorName = "cros-ec-ring"
	// Activity is a special sensor for ChromeOS that produces several kind of
	// activity events by the data of other sensors.
	Activity SensorName = "cros-ec-activity"
)

const (
	// Base means that the sensor is located in the base of the DUT.
	Base SensorLocation = "base"
	// Lid means that the sensor is located in the lid of the DUT.
	Lid SensorLocation = "lid"
	// None means that the sensor location is not known or not applicable.
	None SensorLocation = "none"
)

const (
	// SigMotionID is the ID of the activity, significant motion.
	SigMotionID ActivityID = 1
	// DoubleTapID is the ID of the activity, double tap.
	DoubleTapID ActivityID = 2
	// OrientationID is the ID of the activity, orientation.
	OrientationID ActivityID = 3
	// OnBodyDetectionID is the ID of the activity, on-body detection.
	OnBodyDetectionID ActivityID = 4
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
	Accel:    {},
	Baro:     {},
	Gyro:     {},
	Light:    {},
	Mag:      {},
	Ring:     {},
	Activity: {},
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
	Light: "illuminance",
}

// TriggerType defines the trigger supported by chromeos. sysfs trigger, and ring
// trigger for <= 3.18 kernels.
type TriggerType string

// Trigger : device that can be used to trigger events.
type Trigger struct {
	Type TriggerType
	Name string
}

var iioTriggerRegexp = regexp.MustCompile(`^trigger[0-9]+$`)

// Known type of triggers.
const (
	SysfsTrigger TriggerType = "sysfs"
	RingTrigger  TriggerType = "cros-ec-ring-"
)

var triggerTypes = map[TriggerType]struct{}{
	SysfsTrigger: {},
	RingTrigger:  {},
}

var iioTriggerTypeRegexp = regexp.MustCompile("^(.*)trig.*$")

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

	if _, err := fmt.Sscanf(devName, "iio:device%d", &sensor.IioID); err != nil {
		return nil, errors.Wrapf(err, "%q not a sensor", devName)
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

// GetTriggers enumerates triggers that are exposed by the kernel.
func GetTriggers() ([]*Trigger, error) {
	var ret []*Trigger

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
		trigger, err := parseTrigger(file.Name())
		if err == nil {
			ret = append(ret, trigger)
		}
	}

	return ret, nil
}

// parseTrigger reads the sysfs directory at iioBasePath/devName and returns a
// Trigger if supported..
func parseTrigger(devName string) (*Trigger, error) {
	if !iioTriggerRegexp.MatchString(devName) {
		return nil, errors.New("not a trigger")
	}

	rawName, err := (&Device{devName}).ReadAttr("name")
	if err != nil {
		return nil, errors.Wrap(err, "trigger has no name")
	}

	rawType := iioTriggerTypeRegexp.FindStringSubmatch(rawName)
	if rawType == nil {
		return nil, errors.Errorf("trigger name not understood %s", rawName)
	}

	trigType := TriggerType(rawType[1])
	if _, ok := triggerTypes[trigType]; !ok {
		return nil, errors.Errorf("unknown trigger type %q", trigType)
	}

	return &Trigger{trigType, rawName}, nil
}

func (s *Sensor) readRaw(attr string) (float64, error) {
	possiblePostfix := []string{"raw", "input"}
	rName, _ := readingNames[s.Name]
	for _, postfix := range possiblePostfix {
		var file string
		if attr != "" {
			file = fmt.Sprintf("in_%s_%s_%s", rName, attr, postfix)
		} else {
			file = fmt.Sprintf("in_%s_%s", rName, postfix)
		}
		r, err := s.ReadAttr(file)
		if err == nil {
			return strconv.ParseFloat(strings.TrimSpace(string(r)), 64)
		}
	}
	return 0, errors.Errorf("error reading attribute %v", attr)
}

func (s *Sensor) readAxis() (*SensorReading, error) {
	var ret SensorReading
	ret.Data = make([]float64, 3)
	for axis, prop := range map[string]*float64{
		"x": &ret.Data[0],
		"y": &ret.Data[1],
		"z": &ret.Data[2],
	} {
		reading, err := s.readRaw(axis)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading from sensor %v", s.Name)
		}

		*prop = reading * s.Scale
	}
	return &ret, nil
}

func (s *Sensor) readLight() (*SensorReading, error) {
	var ret SensorReading
	ret.Data = make([]float64, 3)
	for attr, prop := range map[string]*float64{
		"red":   &ret.Data[0],
		"blue":  &ret.Data[1],
		"green": &ret.Data[2],
	} {
		reading, err := s.readRaw(attr)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading from sensor %v", s.Name)
		}

		*prop = reading * s.Scale
	}
	return &ret, nil
}

func (s *Sensor) readSingleAttr() (*SensorReading, error) {
	reading, err := s.readRaw("")
	if err != nil {
		return nil, errors.Wrapf(err, "error reading from sensor %v", s.Name)
	}
	return &SensorReading{Data: []float64{reading * s.Scale}}, nil
}

// Read returns the current readings of the sensor.
func (s *Sensor) Read() (*SensorReading, error) {
	_, ok := readingNames[s.Name]
	if !ok {
		return nil, errors.Errorf("cannot read data from %v", s.Name)
	}

	if s.Name == Accel || s.Name == Gyro {
		return s.readAxis()
	}
	if s.Name == Light {
		reading, err := s.readLight()
		if err != nil {
			return s.readSingleAttr()
		}
		return reading, err
	}
	return nil, errors.Errorf("unsupport sensor %v", s.Name)
}

// WriteAttr writes value to the device's attr file.
func (d *Device) WriteAttr(attr, value string) error {
	if err := ioutil.WriteFile(filepath.Join(basePath, iioBasePath, d.Path, attr),
		[]byte(value), os.ModePerm); err != nil {
		return errors.Wrapf(err, "error writing attribute %q of %v", attr, d.Path)
	}

	return nil
}

// ReadAttr reads the device's attr file and returns the value.
func (d *Device) ReadAttr(attr string) (string, error) {
	a, err := ioutil.ReadFile(filepath.Join(basePath, iioBasePath, d.Path, attr))
	if err != nil {
		return "", errors.Wrapf(err, "error reading attribute %q of %v", attr, d.Path)
	}
	return strings.TrimSpace(string(a)), nil
}

// ReadIntegerAttr reads the device's attr file and returns the integer value.
func (d *Device) ReadIntegerAttr(attr string) (int, error) {
	s, err := d.ReadAttr(attr)
	if err != nil {
		return 0, err
	}
	ret, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Wrapf(err, "the value of %q is not an integer", attr)
	}
	return ret, nil
}
