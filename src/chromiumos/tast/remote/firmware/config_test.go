// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

// These are the names of the mock platforms whose configs we will define.
const (
	myBoardName       = "myboard"
	myParentName      = "myparent"
	myGrandparentName = "mygrandparent"
	myModelName       = "mymodel"
	myOtherModelName  = "myothermodel"
	withECBatteryName = "withECBattery"
)

// In order to tell more easily where each value is obtained from, each mock config sets all numeric fields to the same value.
// Exercise both integers and floats.
const (
	defaultValue       = 1
	myGrandparentValue = 2.2
	myParentValue      = 3.3
	myBoardValue       = 4.4
	myModelValue       = 5.5

	// Duration equivalents of the above
	defaultDuration       = 1 * time.Second
	myGrandparentDuration = 2200 * time.Millisecond
	myParentDuration      = 3300 * time.Millisecond
	myBoardDuration       = 4400 * time.Millisecond
	myModelDuration       = 5500 * time.Millisecond
)

var mockData = map[string]json.RawMessage{
	defaultName: json.RawMessage(fmt.Sprintf(`{
		"platform": null,
		"parent": null,
		"firmware_screen": %d,
		"delay_reboot_to_ping": %d,
		"confirm_screen": %d,
		"usb_plug": %d
	}`, defaultValue, defaultValue, defaultValue, defaultValue)),
	myBoardName: json.RawMessage(fmt.Sprintf(`{
		"platform": %q,
		"parent": %q,
		"firmware_screen": %f,
		"models": {
			%q: {
				"firmware_screen": %f
			}
		}
	}`, myBoardName, myParentName, myBoardValue, myModelName, myModelValue)),
	myParentName: json.RawMessage(fmt.Sprintf(`{
		"platform": %q,
		"parent": %q,
		"confirm_screen": %f,
		"firmware_screen": %f
	}`, myParentName, myGrandparentName, myParentValue, myParentValue)),
	myGrandparentName: json.RawMessage(fmt.Sprintf(`{
		"platform": %q,
		"usb_plug": %f,
		"confirm_screen": %f
	}`, myGrandparentName, myGrandparentValue, myGrandparentValue)),
	withECBatteryName: json.RawMessage(fmt.Sprintf(`{
		"platform": %q,
		"ec_capability": [
			%q
		]
	}`, withECBatteryName, ECBattery)),
}

// setupMockData creates a temporary directory with a consolidated JSON file containing all the data from mockData.
func setupMockData(t *testing.T) (cfgDir, cfgFilepath string, retErr error) {
	// Create JSON bytes out of mock data
	mockJSON, err := json.Marshal(mockData)
	if err != nil {
		return "", "", errors.Wrap(err, "marshaling mock data into JSON")
	}

	// Create temp dir to contain mock consolidated JSON file
	cfgDir = testutil.TempDir(t)
	defer func() {
		if retErr != nil {
			os.RemoveAll(cfgDir)
		}
	}()

	// Create mock consolidated JSON file
	cfgFilepath = filepath.Join(cfgDir, consolidatedBasename)
	if err = ioutil.WriteFile(cfgFilepath, mockJSON, 0644); err != nil {
		return "", "", errors.Wrapf(err, "writing mock json to file %s", cfgFilepath)
	}

	return cfgDir, cfgFilepath, nil
}

// TestNewConfig verifies that we can create a new Config object with proper inheritance.
func TestNewConfig(t *testing.T) {
	cfgDir, cfgFilepath, err := setupMockData(t)
	defer os.RemoveAll(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(cfgFilepath, myBoardName, "")
	if err != nil {
		t.Fatalf("creating config for %s: %+v", myBoardName, err)
	}
	if cfg.Platform != myBoardName {
		t.Errorf("unexpected Platform value; got %q, want %q", cfg.Platform, myBoardName)
	}
	// Platform and parents do not set values; inherit defaults.
	if cfg.DelayRebootToPing != defaultDuration {
		t.Errorf("unexpected DelayRebootToPing value; got %s, want %s", cfg.DelayRebootToPing, defaultDuration)
	}
	// Platform overwrites defaults (even though parent also sets the value)
	if cfg.FirmwareScreen != myBoardDuration {
		t.Errorf("unexpected FirmwareScreen value; got %s, want %s", cfg.FirmwareScreen, myBoardDuration)
	}
	// Platform inherits from parent (even though grandparent also sets the value)
	if cfg.ConfirmScreen != myParentDuration {
		t.Errorf("unexpected ConfirmScreen value; got %s, want %s", cfg.ConfirmScreen, myParentDuration)
	}
	// Platform inherits from grandparent
	if cfg.USBPlug != myGrandparentDuration {
		t.Errorf("unexpected USBPlug value; got %s, want %s", cfg.USBPlug, myGrandparentDuration)
	}
}

// TestNewConfigNoParent verifies that a new config with no parent value has proper inheritance.
func TestNewConfigNoParent(t *testing.T) {
	cfgDir, cfgFilepath, err := setupMockData(t)
	defer os.RemoveAll(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(cfgFilepath, myGrandparentName, "")
	if err != nil {
		t.Fatalf("creating config for %s: %+v", myGrandparentName, err)
	}
	if cfg.Platform != myGrandparentName {
		t.Errorf("unexpected Platform value; got %q, want %q", cfg.Platform, myGrandparentName)
	}
	// mygrandparent has a custom value for USBPlug, overwriting defaults.
	if cfg.USBPlug != myGrandparentDuration {
		t.Errorf("unexpected USBPlug value; got %s, want %s", cfg.USBPlug, myGrandparentDuration)
	}
	// mygrandparent does not set a value for FirmwareScreen, so should inherit defaults.
	if cfg.FirmwareScreen != defaultDuration {
		t.Errorf("unexpected FirmwareScreen value; got %s, want %s", cfg.FirmwareScreen, defaultDuration)
	}
}

// TestNewConfigNoParent verifies that a model-specific config overrides the platform config.
func TestNewConfigModelOverride(t *testing.T) {
	cfgDir, cfgFilepath, err := setupMockData(t)
	defer os.RemoveAll(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	// Test with model-specific override
	cfg, err := NewConfig(cfgFilepath, myBoardName, myModelName)
	if cfg.FirmwareScreen != myModelDuration {
		t.Errorf("unexpected FirmwareScreen value; got %s, want %s // %+v", cfg.FirmwareScreen, myModelDuration, cfg)
	}
	// Test with no model-specific override
	cfg, err = NewConfig(cfgFilepath, myBoardName, myOtherModelName)
	if cfg.FirmwareScreen != myBoardDuration {
		t.Errorf("unexpected FirmwareScreen value; got %s, want %s", cfg.FirmwareScreen, myModelDuration)
	}
}

// TestHasECCapability exercises HasECCapability to verify that we can check whether a Config contains a certain EC capability.
func TestHasECCapability(t *testing.T) {
	cfgDir, cfgFilepath, err := setupMockData(t)
	defer os.RemoveAll(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	// Test a platform that does not define any ec_capability
	cfg, err := NewConfig(cfgFilepath, myBoardName, "")
	if err != nil {
		t.Fatalf("Creating config for platform %s: %+v", myBoardName, err)
	}
	if cfg.HasECCapability(ECBattery) {
		t.Fatalf("Platform %q: HasECCapability(ECBattery) returned True; want False", myBoardName)
	}
	// Test a platform that defines some ec_capabilities
	cfg, err = NewConfig(cfgFilepath, withECBatteryName, "")
	if err != nil {
		t.Fatalf("Creating config for platform %s: %+v", withECBatteryName, err)
	}
	if !cfg.HasECCapability(ECBattery) {
		t.Fatalf("Platform %q: HasECCapability(ECBattery) returned False; want True", withECBatteryName)
	}
	if cfg.HasECCapability(ECPECI) {
		t.Fatalf("Platform %q: HasECCapability(ECPECI) returned True; want False", withECBatteryName)
	}
}
