// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

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

func TestParseAmdS0ixRedidencyFile(t *testing.T) {
	fileName := writeResidencyFile(t, TestAmdS0ixFileContent)
	defer os.Remove(fileName)

	duration, err := getS0ixResidencyStatsFromFiles(fileName, []string{})
	if err != nil {
		t.Fatal(err)
	}

	if duration != 4296355 {
		t.Fatalf("Expected duration %d, actual: %d", 4296355, duration)
	}
}

func TestParseIntelS0ixRedidencyFile(t *testing.T) {
	fileName := writeResidencyFile(t, "321654987")
	defer os.Remove(fileName)

	duration, err := getS0ixResidencyStatsFromFiles("", []string{fileName})
	if err != nil {
		t.Fatal(err)
	}

	if duration != 321654987 {
		t.Fatalf("Expected duration %d, actual: %d", 321654987, duration)
	}
}

func writeResidencyFile(t *testing.T, content string) string {
	file, err := ioutil.TempFile(os.TempDir(), "RedidencyFile-")
	if err != nil {
		t.Fatal(err)
	}

	err = ioutil.WriteFile(file.Name(), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return file.Name()
}
