// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tethering

import (
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/hostapd"
)

// BandEnum is the type for specifying tethering WiFi downstream band.
type BandEnum int

// Band enums.
const (
	Band2p4g BandEnum = iota
	Band5g
	Band6g
)

// Band in string
func (b BandEnum) String() string {
	return []string{"2.4GHz", "5GHz", "6GHz"}[b]
}

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// NoUplink returns an Option which starts standalone downlink Wi-Fi Soft AP only.
func NoUplink(flag bool) Option {
	return func(c *Config) {
		c.NoUL = flag
	}
}

// AutoDisableMin returns an Option which sets the inactive auto disable timer in minutes.
func AutoDisableMin(minute uint32) Option {
	return func(c *Config) {
		c.AutoDisableMin = minute
	}
}

// SSID returns an Option which sets SSID in tethering WiFi downstream config.
func SSID(ssid string) Option {
	return func(c *Config) {
		c.SSID = ssid
	}
}

// Band returns an Option which sets band in tethering WiFi downstream config.
func Band(band BandEnum) Option {
	return func(c *Config) {
		c.Band = band
	}
}

// SecMode returns an Option which sets the security mode in tethering WiFi downstream config.
func SecMode(mode wpa.ModeEnum) Option {
	return func(c *Config) {
		c.SecMode = mode
	}
}

// Cipher returns an Option which sets the security cipher in tethering WiFi downstream config.
func Cipher(cipher wpa.Cipher) Option {
	return func(c *Config) {
		c.Cipher = cipher
	}
}

// PSK returns an Option which sets the pre-shared key in tethering WiFi downstream config.
func PSK(psk string) Option {
	return func(c *Config) {
		c.PSK = psk
	}
}

// Config is the configuration to start tethering session on a DUT.
type Config struct {
	NoUL           bool            // No uplink is used
	AutoDisableMin uint32          // Auto disable time in minutes
	SSID           string          // Downlink Wi-Fi SSID
	Band           BandEnum        // Downlink Wi-Fi band
	SecMode        wpa.ModeEnum    // Downlink Wi-Fi security mode
	Cipher         wpa.Cipher      // Downlink Wi-Fi cipher
	PSK            string          // Downlink Wi-Fi pre-shared key
	SecConf        security.Config // Downlink Wi-Fi security config
}

// NewConfig creates a Config with given options.
// Default value of SSID is a random generated string with prefix "TAST_TEST_" and total length 30.
func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		SSID:    hostapd.RandomSSID("TAST_TEST_"),
		Band:    Band2p4g,
		SecConf: &base.Config{},
	}
	for _, op := range ops {
		op(conf)
	}

	// Generate and validate security config.
	if conf.PSK != "" {
		fac := wpa.NewConfigFactory(conf.PSK, wpa.Mode(conf.SecMode), wpa.Ciphers2(conf.Cipher))
		secConf, err := fac.Gen()
		if err != nil {
			return nil, err
		}
		conf.SecConf = secConf
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return conf, nil
}

// validate validates the Config, c.
func (c *Config) validate() error {
	if c.SSID == "" || len(c.SSID) > 32 {
		return errors.New("invalid SSID")
	}
	if c.SecConf == nil {
		return errors.New("no SecurityConfig set")
	}
	if c.SecConf.Class() != shillconst.SecurityClassPSK && c.SecConf.Class() != shillconst.SecurityNone {
		return errors.Errorf("invalid security class used for tethering: %s", c.SecConf.Class())
	}

	return nil
}
