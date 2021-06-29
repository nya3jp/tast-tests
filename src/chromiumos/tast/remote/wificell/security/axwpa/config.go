// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package axwpa provides a Config type for a wpa network for an AX router.
package axwpa

import (
	sec "chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/router/axrouter"
	"chromiumos/tast/remote/wificell/security"
)

// AxConfig implements security.Config interface for open network, i.e., no security.
type AxConfig struct {
	routerConfigParams []axrouter.ConfigParam
	secCfg             sec.Config
}

// Static check: AxConfig implements security.Config interface.
var _ security.AxConfig = (*AxConfig)(nil)

// RouterParams is a getter that returns the config's router params.
func (r *AxConfig) RouterParams() []axrouter.ConfigParam {
	return r.routerConfigParams
}

// SecConfig is a getter that returns the config's security config.
func (r *AxConfig) SecConfig() sec.Config {
	return r.secCfg
}

// AxConfigFactory provides Gen method to build a new Config.
type AxConfigFactory struct {
	band   axrouter.BandEnum
	psk    string
	cipher axrouter.CipherEnum
	opts   []wpa.Option
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.AxConfigFactory = (*AxConfigFactory)(nil)

// Gen builds a Config.
func (f *AxConfigFactory) Gen() (security.AxConfig, error) {
	routerConfigParams := []axrouter.ConfigParam{{f.band, axrouter.KeyAKM, "psk2"}, {f.band, axrouter.KeyAuthMode, "psk2"}, {f.band, axrouter.KeyWPAPSK, f.psk}}
	if f.cipher == axrouter.AES {
		routerConfigParams = append(routerConfigParams, axrouter.ConfigParam{f.band, axrouter.KeyCrypto, "aes"})
	} else {
		routerConfigParams = append(routerConfigParams, axrouter.ConfigParam{f.band, axrouter.KeyCrypto, "tkip+aes"})
	}
	conf, err := wpa.NewConfigFactory(f.psk, f.opts...).Gen()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't generate inner security config")
	}
	return &AxConfig{routerConfigParams, conf}, nil
}

// NewConfigFactory builds a ConfigFactory.
func NewConfigFactory(band axrouter.BandEnum, psk string, cipher axrouter.CipherEnum, opts ...wpa.Option) *AxConfigFactory {
	return &AxConfigFactory{band, psk, cipher, opts}
}
