// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements Config, a collection of platform-specific attributes used for FW testing.

package firmware

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
)

// ModeSwitcherType represents which methods the platform uses for switching between DUT boot modes.
type ModeSwitcherType string

// Currently, there are exactly three possible values for ModeSwitcherType.
const (
	JetStreamSwitcher        ModeSwitcherType = "jetstream_switcher"
	TabletDetachableSwitcher ModeSwitcherType = "tablet_detachable_switcher"
	KeyboardDevSwitcher      ModeSwitcherType = "keyboard_dev_switcher"
)

// ConfigDir is the basename of the directory within remote/firmware/data/ which contains the JSON files.
const ConfigDir = "fw-testing-configs"

// defaultName is the name of the JSON file containing abstract values, without the .JSON.
const defaultName = "DEFAULTS"

// Config contains platform-specific attributes.
// Fields are documented in autotest/server/cros/faft/configs/DEFAULTS.json.
type Config struct {
	ModeSwitcherType     ModeSwitcherType `json:"mode_switcher_type"`
	PowerButtonDevSwitch bool             `json:"power_button_dev_switch"`
	RecButtonDevSwitch   bool             `json:"rec_button_dev_switch"`
	FirmwareScreen       int              `json:"firmware_screen"`
	DelayRebootToPing    int              `json:"delay_reboot_to_ping"`
	ConfirmScreen        int              `json:"confirm_screen"`
	USBPlug              int              `json:"usb_plug"`
}

// NewConfig creates a new Config matching the DUT platform.
// For now, it only returns default values.
// TODO(b/151469239): Populate with config data matching the DUT platform.
func NewConfig(configDataDir string) (*Config, error) {

	// loadConfigJSON reads '${platform}.json' and loads its contents into a Config struct.
	loadConfigJSON := func(platform string) (*Config, error) {
		fp := filepath.Join(configDataDir, fmt.Sprintf("%s.json", platform))
		b, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, errors.Wrapf(err, "reading datafile %s", fp)
		}
		var cfg *Config
		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshaling json %s from datafile %s", b, fp)
		}
		return cfg, nil
	}

	return loadConfigJSON(defaultName)
}
