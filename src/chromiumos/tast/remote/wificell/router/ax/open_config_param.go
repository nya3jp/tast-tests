// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

// secOpen provides Gen method to build a new Config.
type secOpen struct {
}

// Gen builds a ConfigParam list to allow the router to support an open network flow.
func (f *secOpen) Gen(axType DeviceType, band BandEnum) ([]ConfigParam, error) {
	radio := BandToRadio(axType, band)
	routerConfigParams := []ConfigParam{{radio, KeyAKM, ""}, {radio, KeyAuthMode, "open"}}
	return routerConfigParams, nil
}

// NewSecOpenConfigParamFac builds a SecConfigParamFac for open network for the 802.11ax AP.
func NewSecOpenConfigParamFac() SecConfigParamFac {
	return &secOpen{}
}
