// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
)

const (
	testXattrName = "user.test.xattr"
)

func TestGetSetXattrBytes(t *testing.T) {
	testData := []byte("test")
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	err = SetXattr(file.Name(), testXattrName, testData)
	if err != nil {
		t.Error("Failed to setxattr: ", err)
	}
	data := make([]byte, 0)
	err = GetXattr(file.Name(), testXattrName, &data)
	if err != nil {
		t.Error("Failed to getxattr: ", err)
	}
	if !reflect.DeepEqual(data, testData) {
		t.Errorf("Test data mistmatch! Got: %v Expected: %v", data, testData)
	}
}

func TestGetSetXattrString(t *testing.T) {
	testData := "test"
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	err = SetXattr(file.Name(), testXattrName, testData)
	if err != nil {
		t.Error("Failed to setxattr: ", err)
	}
	data := ""
	err = GetXattr(file.Name(), testXattrName, &data)
	if err != nil {
		t.Error("Failed to getxattr: ", err)
	}
	if !reflect.DeepEqual(data, testData) {
		t.Errorf("Test data mistmatch! Got: %v Expected: %v", data, testData)
	}
}

func TestGetSetXattrBool(t *testing.T) {
	var tcases = []bool{true, false}
	for _, testData := range tcases {
		t.Run(fmt.Sprintf("%t", testData), func(t *testing.T) {
			file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
			if err != nil {
				t.Fatal("Failed to create test file: ", err)
			}
			err = SetXattr(file.Name(), testXattrName, testData)
			if err != nil {
				t.Error("Failed to setxattr: ", err)
			}
			data := false
			err = GetXattr(file.Name(), testXattrName, &data)
			if err != nil {
				t.Error("Failed to getxattr: ", err)
			}
			if data != testData {
				t.Errorf("Test data mistmatch! Got: %v Expected: %v", data, testData)
			}

		})
	}
}

func TestGetInvalidXattrBool(t *testing.T) {
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	const testData = "test_data"
	err = SetXattr(file.Name(), testXattrName, testData)
	if err != nil {
		t.Error("Failed to setxattr: ", err)
	}
	testBoolData := false
	err = GetXattr(file.Name(), testXattrName, &testBoolData)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestGetMissingXattr(t *testing.T) {
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	testBoolData := false
	err = GetXattr(file.Name(), testXattrName, &testBoolData)
	if err == nil {
		t.Error("Expected error")
	}
}
