// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpa

// Option is the function signature used to specify options of Config.
type Option func(*ConfigFactory)

// Mode returns an Option which sets WPA mode in Config.
func Mode(mode ModeEnum) Option {
	return func(f *ConfigFactory) {
		f.blueprint.Mode = mode
	}
}

// CiphersWPA returns an Option which sets the used WPA cipher in Config.
func CiphersWPA(ciphers ...Cipher) Option {
	return func(f *ConfigFactory) {
		f.ciphers = append(f.ciphers, ciphers...)
	}
}

// CiphersWPA2 returns an Option which sets the used WPA2 cipher in Config.
func CiphersWPA2(ciphers ...Cipher) Option {
	return func(f *ConfigFactory) {
		f.ciphers2 = append(f.ciphers2, ciphers...)
	}
}

// PTKRekeyPeriod returns an Option which sets maximum lifetime in seconds for PTK in Config.
func PTKRekeyPeriod(period int) Option {
	return func(f *ConfigFactory) {
		f.blueprint.PTKRekeyPeriod = period
	}
}

// GTKRekeyPeriod returns an Option which sets time interval in seconds for rekeying GTK in Config.
func GTKRekeyPeriod(period int) Option {
	return func(f *ConfigFactory) {
		f.blueprint.GTKRekeyPeriod = period
	}
}

// GMKRekeyPeriod returns an Option which sets time interval in seconds for rekeying GMK in Config.
func GMKRekeyPeriod(period int) Option {
	return func(f *ConfigFactory) {
		f.blueprint.GMKRekeyPeriod = period
	}
}

// UseStrictRekey returns an Option which sets strict rekey in Config.
func UseStrictRekey(use bool) Option {
	return func(f *ConfigFactory) {
		f.blueprint.UseStrictRekey = use
	}
}

// FTMode returns an Option which sets fast transition mode in Config.
func FTMode(ft FTModeEnum) Option {
	return func(f *ConfigFactory) {
		f.blueprint.FTMode = ft
	}
}
