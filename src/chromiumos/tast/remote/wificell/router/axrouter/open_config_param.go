// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// secOpen provides Gen method to build a new Config.
type secOpen struct {
	band BandEnum
}

// Gen builds a ConfigParam list to allow the router to support an open network flow.
func (f *secOpen) Gen() ([]ConfigParam, error) {
	routerConfigParams := []ConfigParam{{f.band, KeyAKM, ""}, {f.band, KeyAuthMode, "open"}}
	return routerConfigParams, nil
}

// NewSecOpenConfigParamFac builds a ConfigFactory for open network for the 802.11ax AP.
func NewSecOpenConfigParamFac(band BandEnum) SecConfigParamFac {
	return &secOpen{band}
}
