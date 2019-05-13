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
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	if err := testutil.WriteFiles(path.Join(td, "sys", "bus", "iio", "devices"),
		map[string]string{
			"bad:device/name":      "bad",
			"iio:device0/name":     "cros-ec-accel",
			"iio:device0/location": "lid",
			"iio:device1/name":     "cros-ec-gyro",
			"iio:device1/location": "base",
			"iio:device2/name":     "cros-ec-unknown",
			"iio:device3/name":     "cros-ec-ring",
		}); err != nil {
		t.Fatal(err)
	}

	oldBasePath := basePath
	basePath = td
	defer func() { basePath = oldBasePath }()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
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
