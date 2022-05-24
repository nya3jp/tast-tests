// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

// secWPA provides Gen method to build a new Config.
type secWPA struct {
	psk    string
	cipher CipherEnum
}

// Gen builds a ConfigParam list to allow the router to support a WPA network flow.
func (f *secWPA) Gen(axType DeviceType, band BandEnum) ([]ConfigParam, error) {
	radio := BandToRadio(axType, band)
	routerConfigParams := []ConfigParam{{radio, KeyAKM, "psk2"}, {radio, KeyAuthMode, "psk2"}, {radio, KeyWPAPSK, f.psk}}
	if f.cipher == AES {
		routerConfigParams = append(routerConfigParams, ConfigParam{radio, KeyCrypto, "aes"})
	} else {
		routerConfigParams = append(routerConfigParams, ConfigParam{radio, KeyCrypto, "tkip+aes"})
	}
	return routerConfigParams, nil
}

// NewSecWPAConfigParamFac builds a SecConfigParamFac for WPA network for the 802.11ax AP.
func NewSecWPAConfigParamFac(psk string, cipher CipherEnum) SecConfigParamFac {
	return &secWPA{psk, cipher}
}
