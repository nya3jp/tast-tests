// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"os"
	"path"
	"reflect"
	"testing"

	"chromiumos/tast/testutil"
)

func TestGetSensors(t *testing.T) {
	defer setupTestFiles(t, map[string]string{
		"bad:device/name":                   "bad",
		"iio:device0/name":                  "cros-ec-accel",
		"iio:device0/location":              "lid",
		"iio:device0/id":                    "0",
		"iio:device0/scale":                 "0.25",
		"iio:device0/frequency":             "100",
		"iio:device0/min_frequency":         "100",
		"iio:device0/max_frequency":         "1000",
		"iio:device1/name":                  "cros-ec-gyro",
		"iio:device1/location":              "base",
		"iio:device1/id":                    "1",
		"iio:device1/buffer/hwfifo_timeout": "0",
		"iio:device2/name":                  "cros-ec-unknown",
		"iio:device3/name":                  "cros-ec-ring",
	})()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
	}

	expected := []*Sensor{
		{Accel, Lid, "iio:device0", 0, .25, 100, 1000, true},
		{Gyro, Base, "iio:device1", 1, 0, 0, 0, false},
		{Ring, None, "iio:device3", 0, 0, 0, 0, false},
	}

	if !reflect.DeepEqual(expected, sensors) {
		t.Errorf("Expected sensors %v but got %v", expected, sensors)
	}
}

func TestNoDeviceDir(t *testing.T) {
	defer setupTestFiles(t, map[string]string{})()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
	}

	if len(sensors) != 0 {
		t.Errorf("Expected no sensors but got %v", sensors)
	}
}

func TestSensorRead(t *testing.T) {
	defer setupTestFiles(t, map[string]string{
		"iio:device0/name":           "cros-ec-accel",
		"iio:device0/location":       "lid",
		"iio:device0/scale":          "0.5",
		"iio:device0/in_accel_x_raw": "10",
		"iio:device0/in_accel_y_raw": "12",
		"iio:device0/in_accel_z_raw": "14",
	})()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
	}

	reading, err := sensors[0].Read()
	if err != nil {
		t.Fatal("Error getting sensor reading: ", err)
	}

	expected := &SensorReading{[]float64{5, 6, 7}, 0, 0, 0}
	if !reflect.DeepEqual(expected, reading) {
		t.Errorf("Unexpected reading: got %v; want %v", reading, expected)
	}
}

func setupTestFiles(t *testing.T, files map[string]string) func() {
	t.Helper()
	td := testutil.TempDir(t)

	if err := testutil.WriteFiles(path.Join(td, "sys/bus/iio/devices"), files); err != nil {
		t.Fatal(err)
	}

	oldBasePath := basePath
	basePath = td
	return func() {
		basePath = oldBasePath
		os.RemoveAll(td)
	}
}
