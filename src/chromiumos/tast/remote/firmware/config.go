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
	"strings"

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

// defaultName is the name of the JSON file containing default config values.
// Although the actual filepath contains a .json extension, this variable does not.
const defaultName = "DEFAULTS"

// configPlatforms is a list of all platforms with JSON files in ConfigDir.
var configPlatforms = []string{
	defaultName,
	"arkham",
	"asuka",
	"atlas",
	"auron",
	"banjo",
	"banon",
	"bob",
	"brain",
	"buddy",
	"candy",
	"caroline",
	"cave",
	"celes",
	"chell",
	"cheza",
	"cid",
	"coral",
	"cyan",
	"dragonegg",
	"drallion",
	"edgar",
	"elm",
	"enguarde",
	"eve",
	"expresso",
	"fievel",
	"fizz",
	"gale",
	"gandof",
	"glados",
	"gnawty",
	"gru",
	"grunt",
	"guado",
	"gus",
	"hana",
	"hatch",
	"heli",
	"jacuzzi",
	"jaq",
	"jecht",
	"jerry",
	"jetstream",
	"kalista",
	"kefka",
	"kevin",
	"kevin-tpm2",
	"kip",
	"kitty",
	"kukui",
	"kunimitsu",
	"lars",
	"lulu",
	"mickey",
	"mighty",
	"minnie",
	"mistral",
	"monroe",
	"nami",
	"nasher",
	"nautilus",
	"ninja",
	"nocturne",
	"nyan",
	"oak",
	"octopus",
	"orco",
	"paine",
	"pinky",
	"poppy",
	"puff",
	"pyro",
	"rambi",
	"rammus",
	"reef",
	"reef_uni",
	"reks",
	"relm",
	"rikku",
	"samus",
	"sand",
	"sarien",
	"scarlet",
	"sentry",
	"setzer",
	"slippy",
	"snappy",
	"soraka",
	"speedy",
	"storm",
	"strago",
	"sumo",
	"swanky",
	"terra",
	"tidus",
	"tiger",
	"trogdor",
	"ultima",
	"umaro",
	"veyron",
	"volteer",
	"whirlwind",
	"winky",
	"wizpig",
	"yuna",
	"zork",
}

// ConfigDatafiles returns the relative paths from data/ to all config files in ConfigDir, as well as to ConfigDir itself.
// It is intended to be used in the Data field of a testing.Test declaration.
func ConfigDatafiles() []string {
	var dfs []string
	for _, platform := range configPlatforms {
		dfs = append(dfs, filepath.Join(ConfigDir, fmt.Sprintf("%s.json", platform)))
	}
	dfs = append(dfs, ConfigDir)
	return dfs
}

// Config contains platform-specific attributes.
// Fields are documented in autotest/server/cros/faft/configs/DEFAULTS.json.
type Config struct {
	Platform             string           `json:"platform"`
	Parent               string           `json:"parent"`
	ModeSwitcherType     ModeSwitcherType `json:"mode_switcher_type"`
	PowerButtonDevSwitch bool             `json:"power_button_dev_switch"`
	RecButtonDevSwitch   bool             `json:"rec_button_dev_switch"`
	FirmwareScreen       int              `json:"firmware_screen"`
	DelayRebootToPing    int              `json:"delay_reboot_to_ping"`
	ConfirmScreen        int              `json:"confirm_screen"`
	USBPlug              int              `json:"usb_plug"`
}

// loadBytes reads '${platform}.json' from configDataDir and returns it as a slice of bytes.
func loadBytes(configDataDir, platform string) ([]byte, error) {
	fp := filepath.Join(configDataDir, fmt.Sprintf("%s.json", platform))
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, errors.Wrapf(err, "reading datafile %s", fp)
	}
	return b, nil
}

// parentFromBytes finds the name of the parent platform referenced by a config's JSON bytes.
func parentFromBytes(b []byte) (string, error) {
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", errors.Wrapf(err, "unmarshaling json bytes %s", b)
	}
	if cfg.Parent == "" && cfg.Platform != "" {
		return defaultName, nil
	}
	return cfg.Parent, nil
}

// NewConfig creates a new Config matching the DUT platform.
// TODO(b/154500336): Load model config and overwrite platform config
func NewConfig(configDataDir, platform string) (*Config, error) {
	// Remove hyphenated suffixes: ex. "samus-kernelnext" becomes "samus"
	platform = strings.SplitN(platform, "-", 2)[0]

	// Load JSON bytes in order from most specific (platform) to most general (DEFAULTS).
	type platformBytes struct {
		name string
		b    []byte
	}
	var inherits []platformBytes
	for platform != "" {
		b, err := loadBytes(configDataDir, platform)
		if err != nil {
			return nil, errors.Wrapf(err, "loading config bytes for platform %s", platform)
		}
		inherits = append(inherits, platformBytes{name: platform, b: b})
		parent, err := parentFromBytes(b)
		if err != nil {
			return nil, errors.Wrapf(err, "determining parent from bytes for %s", platform)
		}
		platform = parent
	}

	// Unmarshal JSON bytes in order from most general (DEFAULTS) to most specific (platform).
	var cfg Config
	for i := len(inherits) - 1; i >= 0; i-- {
		if err := json.Unmarshal(inherits[i].b, &cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal config for %q", inherits[i].name)
		}
	}
	return &cfg, nil
}
