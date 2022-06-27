// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"fmt"
	"os"
	"path"
	"testing"
)

const (
	testStringXattr = "user.test.string"
	testBoolXattr   = "user.test.bool"
)

func TestGetSetXattrString(t *testing.T) {
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	const testData = "test_data"
	err = SetXattrString(file.Name(), testStringXattr, testData)
	if err != nil {
		t.Error("Failed to setxattr: ", err)
	}
	data, err := GetXattrString(file.Name(), testStringXattr)
	if err != nil {
		t.Error("Failed to getxattr: ", err)
	}
	if data != testData {
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
			err = SetXattrBool(file.Name(), testBoolXattr, testData)
			if err != nil {
				t.Error("Failed to setxattr: ", err)
			}
			data, err := GetXattrBool(file.Name(), testBoolXattr)
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
	err = SetXattrString(file.Name(), testBoolXattr, testData)
	if err != nil {
		t.Error("Failed to setxattr: ", err)
	}
	_, err = GetXattrBool(file.Name(), testBoolXattr)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestGetMissingXattr(t *testing.T) {
	file, err := os.Create(path.Join(t.TempDir(), "test.txt"))
	if err != nil {
		t.Fatal("Failed to create test file: ", err)
	}
	_, err = GetXattrBytes(file.Name(), testBoolXattr)
	if err == nil {
		t.Error("Expected error")
	}
	_, err = GetXattrString(file.Name(), testBoolXattr)
	if err == nil {
		t.Error("Expected error")
	}
	_, err = GetXattrBool(file.Name(), testBoolXattr)
	if err == nil {
		t.Error("Expected error")
	}
}
