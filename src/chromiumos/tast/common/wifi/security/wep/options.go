// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wep

// Option is the function signature used to specify options of config.
type Option func(*config)

// DefaultKey returns an Option which sets default key in config.
func DefaultKey(d int) Option {
	return func(c *config) {
		c.defaultKey = d
	}
}

// AuthAlgs returns an Option which sets what authentication algorithm to use in config.
func AuthAlgs(algs ...AuthAlgo) Option {
	return func(c *config) {
		c.authAlgs = 0
		for _, a := range algs {
			c.authAlgs |= a
		}
	}
}
