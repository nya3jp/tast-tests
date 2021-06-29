// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

// axWpaConfigFactory provides Gen method to build a new Config.
type wpaAxConfigFactory struct {
	band   BandEnum
	psk    string
	cipher CipherEnum
}

// Gen builds a ConfigParam list to allow the router to support an open flow.
func (f *wpaAxConfigFactory) Gen() ([]ConfigParam, error) {
	routerConfigParams := []ConfigParam{{f.band, KeyAKM, "psk2"}, {f.band, KeyAuthMode, "psk2"}, {f.band, KeyWPAPSK, f.psk}}
	if f.cipher == AES {
		routerConfigParams = append(routerConfigParams, ConfigParam{f.band, KeyCrypto, "aes"})
	} else {
		routerConfigParams = append(routerConfigParams, ConfigParam{f.band, KeyCrypto, "tkip+aes"})
	}
	return routerConfigParams, nil
}

// NewWpaAxConfigParamFac builds a ConfigFactory.
func NewWpaAxConfigParamFac(band BandEnum, psk string, cipher CipherEnum) AxSecConfigParamFac {
	return &wpaAxConfigFactory{band, psk, cipher}
}
