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
	mockConfigDir := testutil.TempDir(t)
	for platform, b := range mockData {
		err := ioutil.WriteFile(filepath.Join(mockConfigDir, fmt.Sprintf("%s.json", platform)), b, 0644)
		if err != nil {
			return "", errors.Wrapf(err, "writing mock data for platform %s to tempdir %s", platform, mockConfigDir)
		}
	}
	return mockConfigDir, nil
}

func TestLoadBytes(t *testing.T) {
	mockConfigDir, err := setupMockData(t)
	defer os.RemoveAll(mockConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	const p = "myplatform"
	b, err := loadBytes(mockConfigDir, p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, mockData[p]) {
		t.Errorf("unexpected response from loadBytes for platform %s; got %s, want %s", p, b, mockData[p])
	}
}

func TestNewConfig(t *testing.T) {
	mockConfigDir, err := setupMockData(t)
	defer os.RemoveAll(mockConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(mockConfigDir, "myplatform")
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
