// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

// secOwe provides Gen method to build a new Config.
type secOwe struct {
}

// Gen builds a ConfigParam list to allow the router to support an open network flow.
func (f *secOwe) Gen(axType DeviceType, band BandEnum) ([]ConfigParam, error) {
	radio := BandToRadio(axType, band)
	routerConfigParams := []ConfigParam{{radio, KeyAKM, "owe"}, {radio, KeyAuthMode, "owe"}}
	return routerConfigParams, nil
}

// NewSecOweConfigParamFac builds a SecConfigParamFac for the OWE (WiFi Enhanced Open) network for the 802.11ax AP.
func NewSecOweConfigParamFac() SecConfigParamFac {
	return &secOwe{}
}
