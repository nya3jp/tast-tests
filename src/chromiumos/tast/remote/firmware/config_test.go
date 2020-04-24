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

// In order to tell more easily where each value is obtained from, each mock config sets all integer fields to the same value.
const (
	defaultValue       = 1
	myGrandparentValue = 2
	myParentValue      = 3
	myPlatformValue    = 4
)

var mockData = map[string][]byte{
	defaultName: []byte(fmt.Sprintf(`{
		"platform": null,
		"parent": null,
		"firmware_screen": %d,
		"delay_reboot_to_ping": %d,
		"confirm_screen": %d,
		"usb_plug": %d
	}`, defaultValue, defaultValue, defaultValue, defaultValue)),
	"myplatform": []byte(fmt.Sprintf(`{
		"platform": "myplatform",
		"parent": "myparent",
		"firmware_screen": %d
	}`, myPlatformValue)),
	"myparent": []byte(fmt.Sprintf(`{
		"platform": "myparent",
		"parent": "mygrandparent",
		"confirm_screen": %d,
		"firmware_screen": %d
	}`, myParentValue, myParentValue)),
	"mygrandparent": []byte(fmt.Sprintf(`{
		"platform": "mygrandparent",
		"usb_plug": %d,
		"confirm_screen": %d
	}`, myGrandparentValue, myGrandparentValue)),
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
	if cfg.DelayRebootToPing != defaultValue {
		t.Errorf("unexpected DelayRebootToPing value; got %d, want %d", cfg.DelayRebootToPing, defaultValue)
	}
	// Platform overwrites defaults (even though parent also sets the value)
	if cfg.FirmwareScreen != myPlatformValue {
		t.Errorf("unexpected FirmwareScreen value; got %d, want %d", cfg.FirmwareScreen, myPlatformValue)
	}
	// Platform inherits from parent (even though grandparent also sets the value)
	if cfg.ConfirmScreen != myParentValue {
		t.Errorf("unexpected ConfirmScreen value; got %d, want %d", cfg.ConfirmScreen, myParentValue)
	}
	// Platform inherits from grandparent
	if cfg.USBPlug != myGrandparentValue {
		t.Errorf("unexpected USBPlug value; got %d, want %d", cfg.USBPlug, myGrandparentValue)
	}
}

// TestNewConfigNoParent verifies that a new config with no parent value has proper inheritance.
func TestNewConfigNoParent(t *testing.T) {
	mockConfigDir, err := setupMockData(t)
	defer os.RemoveAll(mockConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(mockConfigDir, "mygrandparent")
	if err != nil {
		t.Fatal("creating config for mygrandparent:", err)
	}
	if cfg.Platform != "mygrandparent" {
		t.Errorf(`unexpected Platform value; got %q, want "mygrandparent"`, cfg.Platform)
	}
	// mygrandparent has a custom value for USBPlug, overwriting defaults.
	if cfg.USBPlug != myGrandparentValue {
		t.Errorf("unexpected USBPlug value; got %d, want %d", cfg.USBPlug, myGrandparentValue)
	}
	// mygrandparent does not set a value for FirmwareScreen, so should inherit defaults.
	if cfg.FirmwareScreen != defaultValue {
		t.Errorf("unexpected FirmwareScreen value; got %d, want %d", cfg.FirmwareScreen, defaultValue)
	}
}
