// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

import "chromiumos/tast/errors"

// secSAE provides Gen method to build a new Config.
type secSAE struct {
	psk    string
	cipher CipherEnum
}

// Gen builds a ConfigParam list to allow the router to support a WPA network flow.
func (f *secSAE) Gen(axType DeviceType, band BandEnum) ([]ConfigParam, error) {
	radio := BandToRadio(axType, band)
	routerConfigParams := []ConfigParam{{radio, KeyAKM, "sae"}, {radio, KeyAuthMode, "sae"}, {radio, KeyWPAPSK, f.psk}}
	if f.cipher == AES {
		routerConfigParams = append(routerConfigParams, ConfigParam{radio, KeyCrypto, "aes"})
	} else {
		return nil, errors.New("crypto schema not supported")
	}
	return routerConfigParams, nil
}

// NewSecSAEConfigParamFac builds a SecConfigParamFac for WPA3-SAE network for the 802.11ay AP.
func NewSecSAEConfigParamFac(psk string, cipher CipherEnum) SecConfigParamFac {
	return &secSAE{psk, cipher}
}
