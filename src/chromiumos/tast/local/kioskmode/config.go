// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kioskmode

import "chromiumos/tast/common/policy"

// Option is a self-referential function can be used to configure Kiosk mode.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *MutableConfig) error

// Config contains configurations for Kiosk mode. It holds the necessary
// policies that will be set to enable Kiosk mode.
// Once retrieved by NewConfig() it should be used to read from not to modify.
type Config struct {
	m MutableConfig
}

// MutableConfig holds pieces of configuration that are set with Options.
type MutableConfig struct {
	// DeviceLocalAccounts is the configuration with local device accounts.
	DeviceLocalAccounts *policy.DeviceLocalAccounts
	// ExtraPolicies holds extra policies that will be applied.
	ExtraPolicies []policy.Policy
	// TODO: kamilszarek add support for the public account policies together
	// with a test.
	// PublicAccountID is an id of the account to which PublicAccountPolicies
	// will be applied.
	PublicAccountID *string
	// PublicAccountPolicies are policies that will be applied to the account
	// with PublicAccountID id.
	PublicAccountPolicies []policy.Policy
	// AutoLaunch determines whether to set Kiosk mode to autolaunch. When true
	// AutoLaunchKioskAppID id is set to autolaunch.
	AutoLaunch bool
	// AutoLaunchKioskAppID is an id of the autolaunched Kiosk account.
	AutoLaunchKioskAppID *string
}

// NewConfig creates new configuration.
func NewConfig(opts []Option) (*Config, error) {
	cfg := &Config{
		m: MutableConfig{},
	}
	for _, opt := range opts {
		if err := opt(&cfg.m); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}
