// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// secWPA provides Gen method to build a new Config.
type secWPA struct {
	band   BandEnum
	psk    string
	cipher CipherEnum
}

// Gen builds a ConfigParam list to allow the router to support a WPA network flow.
func (f *secWPA) Gen() ([]ConfigParam, error) {
	routerConfigParams := []ConfigParam{{f.band, KeyAKM, "psk2"}, {f.band, KeyAuthMode, "psk2"}, {f.band, KeyWPAPSK, f.psk}}
	if f.cipher == AES {
		routerConfigParams = append(routerConfigParams, ConfigParam{f.band, KeyCrypto, "aes"})
	} else {
		routerConfigParams = append(routerConfigParams, ConfigParam{f.band, KeyCrypto, "tkip+aes"})
	}
	return routerConfigParams, nil
}

// NewSecWPAConfigParamFac builds a SecConfigParamFac for WPA network for the 802.11ax AP.
func NewSecWPAConfigParamFac(band BandEnum, psk string, cipher CipherEnum) SecConfigParamFac {
	return &secWPA{band, psk, cipher}
}
