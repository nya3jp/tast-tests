// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

var mockData = map[string][]byte{
	defaultName: []byte(`{
		"platform": "DEFAULTS",
		"parent": null,
		"firmware_screen": 1
	}`),
	"myplatform": []byte(`{
		"platform": "myplatform",
		"firmware_screen": 2
	}`),
}

// setupMockData creates a temporary directory containing .json files for each platform in mockData.
func setupMockData(t *testing.T) (string, error) {
	tempDir := testutil.TempDir(t)
	for platform, b := range mockData {
		err := ioutil.WriteFile(filepath.Join(tempDir, fmt.Sprintf("%s.json", platform)), b, 0644)
		if err != nil {
			return "", errors.Wrapf(err, "writing mock data for platform %s to tempdir %s", platform, tempDir)
		}
		if platform != defaultName {
			configPlatforms = append(configPlatforms, "myplatform")
		}
	}
	return tempDir, nil
}

// teardownMockData cleans up the temporary directory containing mock config data.
func teardownMockData(tempDir string) error {
	return os.RemoveAll(tempDir)
}

// TestMockData verifies that unit tests can correctly set up and tear down mock datafiles.
func TestMockData(t *testing.T) {
	tempDir, err := setupMockData(t)
	defer teardownMockData(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// pathExists returns true iff the path can be found on the filesystem.
	pathExists := func(fp string) bool {
		_, err := os.Stat(fp)
		return !os.IsNotExist(err)
	}
	if !pathExists(tempDir) {
		t.Fatal("failed to find tempDir:", tempDir)
	}

	// mockDatafile returns the path to the platform's mock datafile in tempDir.
	mockDatafile := func(platform string) string {
		return filepath.Join(tempDir, fmt.Sprintf("%s.json", platform))
	}
	for platform, b := range mockData {
		fp := mockDatafile(platform)
		if !pathExists(fp) {
			t.Error("failed to find mock datafile:", fp)
			continue
		}
		contents, err := ioutil.ReadFile(fp)
		if err != nil {
			t.Errorf("failed to read contents from mock datafile %s: %v", fp, err)
		} else if !bytes.Equal(contents, b) {
			t.Errorf("unexpected contents of mock datafile %s.json; got %s, want %s", platform, contents, b)
		}
	}

	if err = teardownMockData(tempDir); err != nil {
		t.Fatal(err)
	}
	if pathExists(tempDir) {
		t.Errorf("unexpectedly found tempDir %s after teardown", tempDir)
	}
	if df := mockDatafile(defaultName); pathExists(df) {
		t.Errorf("unexpectedly found %s after teardown", df)
	}
}

func TestLoadBytes(t *testing.T) {
	tempDir, err := setupMockData(t)
	defer teardownMockData(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	const p = "myplatform"
	b, err := loadBytes(tempDir, p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, mockData[p]) {
		t.Errorf("unexpected response from loadBytes for platform %s; got %s, want %s", p, b, mockData[p])
	}
}

func TestNewConfig(t *testing.T) {
	tempDir, err := setupMockData(t)
	defer teardownMockData(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(tempDir, "myplatform")
	if err != nil {
		t.Error("creating config for myplatform:", err)
	}
	if cfg.Platform != "myplatform" {
		t.Errorf(`unexpected Platform value; got %q, want "myplatform"`, cfg.Platform)
	}
	if cfg.FirmwareScreen != 2 {
		t.Errorf("unexpected FirmwareScreen value; got %d, want 2", cfg.FirmwareScreen)
	}
}
