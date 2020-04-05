// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpa

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// Mode returns an Option which sets WPA mode in Config.
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		c.Mode = mode
	}
}

// Ciphers returns an Option which sets the used WPA ciphers in Config.
func Ciphers(ciphers ...Cipher) Option {
	return func(c *Config) {
		c.Ciphers = append(c.Ciphers, ciphers...)
	}
}

// Ciphers2 returns an Option which sets the used WPA2 ciphers in Config.
func Ciphers2(ciphers ...Cipher) Option {
	return func(c *Config) {
		c.Ciphers2 = append(c.Ciphers2, ciphers...)
	}
}

// PTKRekeyPeriod returns an Option which sets maximum lifetime in seconds for PTK in Config.
func PTKRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.PTKRekeyPeriod = period
	}
}

// GTKRekeyPeriod returns an Option which sets time interval in seconds for rekeying GTK in Config.
func GTKRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.GTKRekeyPeriod = period
	}
}

// GMKRekeyPeriod returns an Option which sets time interval in seconds for rekeying GMK in Config.
func GMKRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.GMKRekeyPeriod = period
	}
}

// UseStrictRekey returns an Option which sets strict rekey in Config.
func UseStrictRekey(use bool) Option {
	return func(c *Config) {
		c.UseStrictRekey = use
	}
}

// FTMode returns an Option which sets fast transition mode in Config.
func FTMode(ft FTModeEnum) Option {
	return func(c *Config) {
		c.FTMode = ft
	}
}
