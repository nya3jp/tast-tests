// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements Config, a collection of platform-specific attributes used for FW testing.

package firmware

// ModeSwitcherType represents which methods the platform uses for switching between DUT boot modes.
type ModeSwitcherType string

// Currently, there are exactly three possible values for ModeSwitcherType.
const (
	JetStreamSwitcher        ModeSwitcherType = "jetstream_switcher"
	TabletDetachableSwitcher ModeSwitcherType = "tablet_detachable_switcher"
	KeyboardDevSwitcher      ModeSwitcherType = "keyboard_dev_switcher"
)

// Config contains platform-specific attributes.
// Fields are documented in autotest/server/cros/faft/configs/DEFAULTS.json.
type Config struct {
	ModeSwitcherType     ModeSwitcherType
	PowerButtonDevSwitch bool
	RecButtonDevSwitch   bool
	FirmwareScreen       int
	DelayRebootToPing    int
	ConfirmScreen        int
	USBPlug              int
}

// NewConfig creates a new Config matching the DUT platform.
// Normally these attributes would come from JSON files, but those files have not been added to Tast yet.
// For now, it instead returns default values.
// TODO(b/151469239): Populate with real, platform-specific config data.
func NewConfig() (*Config, error) {
	cfg := &Config{
		ModeSwitcherType:     KeyboardDevSwitcher,
		PowerButtonDevSwitch: false,
		RecButtonDevSwitch:   false,
		FirmwareScreen:       10,
		DelayRebootToPing:    30,
		ConfirmScreen:        3,
		USBPlug:              10,
	}
	return cfg, nil
}
