// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wep

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// DefaultKey returns an Option which sets default key in Config.
func DefaultKey(d int) Option {
	return func(c *Config) {
		c.defaultKey = d
	}
}

// AuthAlgs returns an Option which sets what authentication algorithm to use in Config.
func AuthAlgs(algs ...AuthAlgo) Option {
	return func(c *Config) {
		c.authAlgs = 0
		for _, a := range algs {
			c.authAlgs |= a
		}
	}
}
