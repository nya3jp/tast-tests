// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"io/ioutil"
	"os"
	"testing"
)

const TestAmdS0ixFileContent = `=== S0ix statistics ===
S0ix Entry Time: 9326053598
S0ix Exit Time: 9532278675
Residency Time: 4296355
`

func TestParseInvalidFileList(t *testing.T) {
	_, err := getS0ixResidencyStatsFromFiles("", []string{})
	if err == nil {
		t.Fatal("Test didn't report error.")
	}
}

func TestParseAmdS0ixResidencyFile(t *testing.T) {
	fileName := writeResidencyFile(t, TestAmdS0ixFileContent)
	defer os.Remove(fileName)

	duration, err := getS0ixResidencyStatsFromFiles(fileName, []string{})
	if err != nil {
		t.Fatal(err)
	}

	if duration != 4296355 {
		t.Fatalf("Actual duration %d, expected: %d", duration, 4296355)
	}
}

func TestParseIntelS0ixResidencyFile(t *testing.T) {
	fileName := writeResidencyFile(t, "321654987")
	defer os.Remove(fileName)

	duration, err := getS0ixResidencyStatsFromFiles("", []string{fileName})
	if err != nil {
		t.Fatal(err)
	}

	if duration != 321654987 {
		t.Fatalf("Actual duration %d, expected: %d", duration, 321654987)
	}
}

func TestParseS2IdleResidencyFile(t *testing.T) {
	fileName1 := writeResidencyFile(t, "123")
	defer os.Remove(fileName1)
	fileName2 := writeResidencyFile(t, "456")
	defer os.Remove(fileName2)

	duration, err := getS2IdleResidencyStats(os.TempDir() + "/ResidencyFile-*")
	if err != nil {
		t.Fatal(err)
	}

	// duration is expected as 123 + 456 = 579.
	if duration != 579 {
		t.Fatalf("Actual duration %d, expected: %d", duration, 579)
	}
}

func TestParseS2IdleResidencyInvalidFile(t *testing.T) {
	_, err := getS2IdleResidencyStats(os.TempDir() + "/ResidencyFile-*")
	if err == nil {
		t.Fatal("Test didn't report error.")
	}
}

func writeResidencyFile(t *testing.T, content string) string {
	file, err := ioutil.TempFile(os.TempDir(), "ResidencyFile-")
	if err != nil {
		t.Fatal(err)
	}

	err = ioutil.WriteFile(file.Name(), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return file.Name()
}
