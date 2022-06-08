// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
)

// Option is a self-referential function can be used to configure MGS mode.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *MutableConfig) error

// Config contains configurations for MGS mode. It holds the necessary
// policies that will be set to enable MGS mode.
// Once retrieved by NewConfig() it should be used to read from not to modify.
type Config struct {
	m MutableConfig
}

// MutableConfig holds pieces of configuration that are set with Options.
type MutableConfig struct {
	// MGSAccounts applies DeviceLocalAccountInfo custom DeviceLocalAccountInfo configuration.
	MGSAccounts *policy.DeviceLocalAccounts
	// ExtraPolicies holds extra policies that will be applied.
	ExtraPolicies []policy.Policy
	// ExternalPolicyBlob allows to specify a policy blob that is constructed externally.
	ExternalPolicyBlob *policy.Blob
	// PublicAccountPolicies holds public accounts' IDs with associated polices
	// that will be applied to the them.
	PublicAccountPolicies map[string][]policy.Policy
	// AutoLaunch determines whether to set MGS mode to autolaunch. When true
	// AutoLaunchMGSAppID id is set to autolaunch.
	AutoLaunch bool
	// AutoLaunchMGSAppID is an id of the autolaunched MGS account.
	AutoLaunchMGSAppID *string
	// ExtraChromeOptions holds all extra options that will be passed to Chrome
	// instance that will run in MGS mode.
	ExtraChromeOptions []chrome.Option
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
