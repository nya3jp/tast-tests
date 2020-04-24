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
		"platform": null,
		"parent": null,
		"firmware_screen": 1,
		"delay_reboot_to_ping": 1,
		"confirm_screen": 1,
		"usb_plug": 1
	}`),
	"myplatform": []byte(`{
		"platform": "myplatform",
		"parent": "myparent",
		"firmware_screen": 2
	}`),
	"myparent": []byte(`{
		"platform": "myparent",
		"parent": "mygrandparent",
		"confirm_screen": 3,
		"firmware_screen": 3
	}`),
	"mygrandparent": []byte(`{
		"platform": "mygrandparent",
		"usb_plug": 4,
		"confirm_screen": 4
	}`),
}

// setupMockData creates a temporary directory containing .json files for each platform in mockData.
func setupMockData(t *testing.T) (string, error) {
	mockConfigDir := testutil.TempDir(t)
	for platform, b := range mockData {
		err := ioutil.WriteFile(filepath.Join(mockConfigDir, fmt.Sprintf("%s.json", platform)), b, 0644)
		if err != nil {
			return mockConfigDir, errors.Wrapf(err, "writing mock data for platform %s to tempdir %s", platform, mockConfigDir)
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

// TestNewConfig verifies that we can create a new Config object with proper inheritance.
func TestNewConfig(t *testing.T) {
	mockConfigDir, err := setupMockData(t)
	defer os.RemoveAll(mockConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(mockConfigDir, "myplatform")
	if err != nil {
		t.Fatal("creating config for myplatform:", err)
	}
	if cfg.Platform != "myplatform" {
		t.Errorf(`unexpected Platform value; got %q, want "myplatform"`, cfg.Platform)
	}
	// Platform and parents do not set values; inherit defaults.
	if cfg.DelayRebootToPing != 1 {
		t.Errorf("unexpected DelayRebootToPing value; got %d, want 1", cfg.DelayRebootToPing)
	}
	// Platform overwrites defaults (even though parent also sets the value)
	if cfg.FirmwareScreen != 2 {
		t.Errorf("unexpected FirmwareScreen value; got %d, want 2", cfg.FirmwareScreen)
	}
	// Platform inherits from parent (even though grandparent also sets the value)
	if cfg.ConfirmScreen != 3 {
		t.Errorf("unexpected ConfirmScreen value; got %d, want 1", cfg.ConfirmScreen)
	}
	// Platform inherits from grandparent
	if cfg.USBPlug != 4 {
		t.Errorf("unexpected USBPlug value; got %d, want 1", cfg.USBPlug)
	}
}
