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
	"time"

	"chromiumos/tast/errors"
)

// ModeSwitcherType represents which methods the platform uses for switching between DUT boot modes.
type ModeSwitcherType string

// Currently, there are exactly four possible values for ModeSwitcherType.
const (
	JetStreamSwitcher        ModeSwitcherType = "jetstream_switcher"
	KeyboardDevSwitcher      ModeSwitcherType = "keyboard_dev_switcher"
	MenuSwitcher             ModeSwitcherType = "menu_switcher"
	TabletDetachableSwitcher ModeSwitcherType = "tablet_detachable_switcher"
)

// ECCapability represents one feature that the EC can control.
type ECCapability string

// These are the ECCapabilities currently described in fw-testing-configs.
const (
	ECADCECTemp      ECCapability = "adc_ectemp"
	ECARM            ECCapability = "arm"
	ECBattery        ECCapability = "battery"
	ECCBI            ECCapability = "cbi"
	ECCharging       ECCapability = "charging"
	ECDoubleBoot     ECCapability = "doubleboot"
	ECKeyboard       ECCapability = "keyboard"
	ECLid            ECCapability = "lid"
	ECPECI           ECCapability = "peci"
	ECSmartUSBCharge ECCapability = "smart_usb_charge"
	ECThermal        ECCapability = "thermal"
	ECUSB            ECCapability = "usb"
	ECUSBPDUART      ECCapability = "usbpd_uart"
	ECX86            ECCapability = "x86"
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
	"coral",
	"cyan",
	"dedede",
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
	"kevin_tpm2",
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
	ECCapability         []ECCapability   `json:"ec_capability"`
	ModeSwitcherType     ModeSwitcherType `json:"mode_switcher_type"`
	PowerButtonDevSwitch bool             `json:"power_button_dev_switch"`
	RecButtonDevSwitch   bool             `json:"rec_button_dev_switch"`

	// Raw duration fields represent a quantity of seconds.
	// They are used during NewConfig to populate actual duration fields, which are defined below.
	// Generally, these raw fields should not be accessed by tests and libraries.
  	// Nevertheless, the raw fields must be exported in order for them to be set by json.Unmarshal.
	RawConfirmScreen     float64 `json:"confirm_screen"`
	RawDelayRebootToPing float64 `json:"delay_reboot_to_ping"`
	RawECBootToPwrButton float64 `json:"ec_boot_to_pwr_button"`
	RawFirmwareScreen    float64 `json:"firmware_screen"`
	RawUSBPlug           float64 `json:"usb_plug"`

	// Actual duration fields are populated during NewConfig based on raw JSON duration values, defined above.
	ConfirmScreen     time.Duration
	DelayRebootToPing time.Duration
	ECBootToPwrButton time.Duration
	FirmwareScreen    time.Duration
	USBPlug           time.Duration

	// Models maps DUT model names to overriding config JSON objects.
	Models map[string]json.RawMessage `json:"models"`
}

// CfgPlatformFromLSBBoard interprets a board name that would come from /etc/lsb-release, and returns the name of the platform whose config should be loaded.
func CfgPlatformFromLSBBoard(board string) string {
	// Remove hyphenated suffixes: ex. "samus-kernelnext" becomes "samus"
	board = strings.SplitN(board, "-", 2)[0]
	// If the board name is given as board_variant, take just the variant: ex. "veyron_minnie" becomes "minnie"
	board = strings.Split(board, "_")[strings.Count(board, "_")]
	return board
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
func NewConfig(configDataDir, board, model string) (*Config, error) {
	// Load JSON bytes in order from most specific (board) to most general (DEFAULTS).
	type platformBytes struct {
		name string
		b    []byte
	}
	var inherits []platformBytes
	for platform := CfgPlatformFromLSBBoard(board); platform != ""; {
		b, err := loadBytes(configDataDir, platform)
		if err != nil {
			return nil, errors.Wrapf(err, "loading config bytes for platform %s", platform)
		}
		parent, err := parentFromBytes(b)
		if err != nil {
			return nil, errors.Wrapf(err, "determining parent from bytes for %s", platform)
		}
		inherits = append(inherits, platformBytes{name: platform, b: b})
		platform = parent
	}

	// Unmarshal JSON bytes in order from most general (DEFAULTS) to most specific (board).
	var cfg Config
	for i := len(inherits) - 1; i >= 0; i-- {
		if err := json.Unmarshal(inherits[i].b, &cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal config for %q", inherits[i].name)
		}
	}

	// Unmarshal model-level config on top of the existing config.
	// Models are only expected to be defined in the lowest-level (board) config files, not in parent config files.
	if modelCfg, ok := cfg.Models[model]; ok {
		if err := json.Unmarshal(modelCfg, &cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal model-level config for %q", model)
		}
	}

	// Populate actual durations based on raw JSON values.
	cfg.ConfirmScreen = toSeconds(cfg.RawConfirmScreen)
	cfg.DelayRebootToPing = toSeconds(cfg.RawDelayRebootToPing)
	cfg.ECBootToPwrButton = toSeconds(cfg.RawECBootToPwrButton)
	cfg.FirmwareScreen = toSeconds(cfg.RawFirmwareScreen)
	cfg.USBPlug = toSeconds(cfg.RawUSBPlug)

	return &cfg, nil
}

// HasECCapability checks whether cfg has a certain ECCapability.
func (cfg *Config) HasECCapability(ecc ECCapability) bool {
	for _, capability := range cfg.ECCapability {
		if ecc == capability {
			return true
		}
	}
	return false
}

// toSeconds casts a float64 to a time.Duration, in seconds.
func toSeconds(f float64) time.Duration {
  	// The 1000* factor enables a non-integer f to be cast as a time.Duration, an integer field.
	return time.Duration(1000*f) * time.Millisecond
}
