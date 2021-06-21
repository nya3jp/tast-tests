// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dutcfg provides utilities for controlling the DUT.
package dutcfg

import "chromiumos/tast/common/wifi/security"

type ConnConfig struct {
	Ssid    string
	Hidden  bool
	SecConf security.Config
	Props   map[string]interface{}
}

// ConnOption is the function signature used to modify ConnectWifi.
type ConnOption func(*ConnConfig)

// ConnHidden returns a ConnOption which sets the hidden property.
func ConnHidden(h bool) ConnOption {
	return func(c *ConnConfig) {
		c.Hidden = h
	}
}

// ConnSecurity returns a ConnOption which sets the security configuration.
func ConnSecurity(s security.Config) ConnOption {
	return func(c *ConnConfig) {
		c.SecConf = s
	}
}

// ConnProperties returns a ConnOption which sets the service properties.
func ConnProperties(p map[string]interface{}) ConnOption {
	return func(c *ConnConfig) {
		c.Props = make(map[string]interface{})
		for k, v := range p {
			c.Props[k] = v
		}
	}
}
