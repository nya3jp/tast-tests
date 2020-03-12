// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements Config, a collection of platform-specific attributes.
*/

// Config is a collection of platform-specific attributes.
// The fields are documented in the JSON config files.
type Config struct {
	ModeSwitcherType     string
	PowerButtonDevSwitch bool
	RecButtonDevSwitch   bool
	FirmwareScreen       int
	DelayRebootToPing    int
	ConfirmScreen        int
	USBPlug              int
}

// There are exactly three possible values for ModeSwitcherType.
const (
	JetStreamSwitcher        = "jetstream_switcher"
	TabletDetachableSwitcher = "tablet_detachable_switcher"
	KeyboardDevSwitcher      = "keyboard_dev_switcher"
)

// NewConfig creates a new Config matching the DUT platform.
// For now, because config JSON files are still being added, it instead returns default values.
func NewConfig() *Config {
	return &Config{
		ModeSwitcherType:     KeyboardDevSwitcher,
		PowerButtonDevSwitch: false,
		RecButtonDevSwitch:   false,
		FirmwareScreen:       10,
		DelayRebootToPing:    30,
		ConfirmScreen:        3,
		USBPlug:              10,
	}
}
