// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// axOpenConfigFactory provides Gen method to build a new Config.
type openAxConfigParamFac struct {
	band BandEnum
}

// Gen builds a ConfigParam list to allow the router to support a WPA flow.
func (f *openAxConfigParamFac) Gen() ([]ConfigParam, error) {
	routerConfigParams := []ConfigParam{{f.band, KeyAKM, ""}, {f.band, KeyAuthMode, "open"}}
	return routerConfigParams, nil
}

// NewOpenAxConfigParamFac builds a ConfigFactory.
func NewOpenAxConfigParamFac(band BandEnum) AxSecConfigParamFac {
	return &openAxConfigParamFac{band}
}
