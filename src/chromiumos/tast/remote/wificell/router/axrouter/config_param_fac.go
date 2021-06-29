// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// Security determines the PSK authentication method used by the test
type Security int

const (
	// SecOpen is for an open (no password) flow
	SecOpen Security = iota
	// SecWPA is for the WPA2-PSK flow
	SecWPA
)

// SecConfigParamFac defines a Gen() method to generate a ConfigParam list.
type SecConfigParamFac interface {
	// Gen builds a list of ConfigParam.
	Gen() ([]ConfigParam, error)
}
