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
	ModeSwitcherType     ModeSwitcherType `json:"mode_switcher_type"`
	PowerButtonDevSwitch bool             `json:"power_button_dev_switch"`
	RecButtonDevSwitch   bool             `json:"rec_button_dev_switch"`
	FirmwareScreen       int              `json:"firmware_screen"`
	DelayRebootToPing    int              `json:"delay_reboot_to_ping"`
	ConfirmScreen        int              `json:"confirm_screen"`
	USBPlug              int              `json:"usb_plug"`
}

// ConfigFactory is an interface to create Config objects.
// It contains a function, loadBytes, which finds the raw bytes for a given platform, which can then be unmarshalled into a Config struct.
// ConfigFactory.NewConfig calls loadBytes several times in order to implement the config inheritance model.
type ConfigFactory struct {
	loadBytes func(platform string) ([]byte, error)
}

// NewConfigFactory creates a ConfigFactory which creates configs based on JSON files in configDataDir.
func NewConfigFactory(configDataDir string) *ConfigFactory {
	// bytesFromDataFile reads '${platform}.json' from configDataDir and loads its contents into a Config struct.
	bytesFromDataFile := func(platform string) ([]byte, error) {
		cfgFound := false
		for _, p := range configPlatforms {
			if platform == p {
				cfgFound = true
				break
			}
		}
		if !cfgFound {
			return nil, errors.Errorf("configPlatforms in remote/firmware/config.go does not contain platform %q", platform)
		}
		fp := filepath.Join(configDataDir, fmt.Sprintf("%s.json", platform))
		b, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, errors.Wrapf(err, "reading datafile %s", fp)
		}
		return b, nil
	}
	return &ConfigFactory{loadBytes: bytesFromDataFile}
}

// NewConfig creates a new Config matching the DUT platform.
// TODO(b/151469239): Use the platform config to overwrite parent config(s) and default config
// TODO(b/151469239): Load model config and overwrite platform config
func (cf *ConfigFactory) NewConfig(platform string) (*Config, error) {
	b, err := cf.loadBytes(platform)
	if err != nil {
		return nil, errors.Wrapf(err, "loading config bytes for platform %s", platform)
	}
	var cfg *Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling json bytes %s for platform %s", b, platform)
	}
	return cfg, nil
}
