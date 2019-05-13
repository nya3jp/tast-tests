// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
)

type file struct {
	name     string
	contents string
}

type dir struct {
	name  string
	dirs  []dir
	files []file
}

var iioData = dir{name: "testdata", dirs: []dir{
	dir{name: "sys", dirs: []dir{
		dir{name: "bus", dirs: []dir{
			dir{name: "iio", dirs: []dir{
				dir{name: "devices", dirs: []dir{
					dir{name: "bad:device", files: []file{
						file{name: "name", contents: "bad"},
					}},
					dir{name: "iio:device0", files: []file{
						file{name: "name", contents: "cros-ec-accel"},
						file{name: "location", contents: "lid"},
					}},
					dir{name: "iio:device1", files: []file{
						file{name: "name", contents: "cros-ec-gyro"},
						file{name: "location", contents: "base"},
					}},
					dir{name: "iio:device2", files: []file{
						file{name: "name", contents: "cros-ec-unknown"},
					}},
					dir{name: "iio:device3", files: []file{
						file{name: "name", contents: "cros-ec-ring"},
					}},
				}},
			}},
		}},
	}},
}}

func TestGetSensors(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("Cannot get pwd")
	}

	buildTestData(root, &iioData, t)
	defer func() { os.RemoveAll(path.Join(root, iioData.name)) }()

	abs, err := filepath.Abs(iioData.name)
	if err != nil {
		t.Fatalf("Cannot find directory %v", iioData.name)
	}

	oldBasePath := basePath
	basePath = abs
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

func buildTestData(root string, d *dir, t *testing.T) {
	pwd := path.Join(root, d.name)
	os.RemoveAll(pwd)
	if err := os.Mkdir(pwd, os.ModePerm); err != nil {
		t.Fatalf("Cannot create dir %v: %v", pwd, err)
	}

	for _, child := range d.dirs {
		buildTestData(pwd, &child, t)
	}

	for _, f := range d.files {
		file, err := os.Create(path.Join(pwd, f.name))
		if err != nil {
			t.Fatalf("Cannot create file %v: %v", f.name, err)
		}

		file.WriteString(f.contents)

		err = file.Close()
	}
}
