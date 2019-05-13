// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetSensors(t *testing.T) {
	abs, err := filepath.Abs("testdata")

	if err != nil {
		t.Fatalf("Cannot find testdata directory")
	}

	basePath = abs

	sensors, err := GetSensors()

	if err != nil {
		t.Fatalf("Error getting sensors")
	}

	expected := []Sensor{
		{Accel, Lid, "iio:device0"},
		{Gyro, Base, "iio:device1"},
		{Ring, None, "iio:device3"},
	}

	if !reflect.DeepEqual(expected, sensors) {
		t.Errorf("Expected sensors %v but got %v", expected, sensors)
	}
}
