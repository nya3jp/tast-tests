// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// AxSecurity determines the PSK authentication method used by the test
type AxSecurity int

const (
	// Open is for an open (no password) flow
	Open AxSecurity = iota
	// WPA is for the WPA2-PSK flow
	WPA
)

// AxSecConfigParamFac defines a Gen() method to generate a ConfigParam list.
type AxSecConfigParamFac interface {
	// Gen builds a ConfigParamList.
	Gen() ([]ConfigParam, error)
}
