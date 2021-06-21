// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axwpa

import (
	sec "chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/security"
)

// AxConfig implements security.Config interface for open network, i.e., no security.
type AxConfig struct {
	routerConfigParams []router.AxRouterConfigParam
	secCfg             sec.Config
}

// Static check: AxConfig implements security.Config interface.
var _ security.AxConfig = (*AxConfig)(nil)

// AxConfigFactory provides Gen method to build a new Config.
type AxConfigFactory struct {
	band   router.BandEnum
	psk    string
	cipher router.CipherEnum
	opts   []wpa.Option
}

// Gen builds a Config.
func (f *AxConfigFactory) Gen() (security.AxConfig, error) {
	routerConfigParams := []router.AxRouterConfigParam{{f.band, router.KeyAkm, "psk2"}, {f.band, router.KeyAuthMode, "psk2"}, {f.band, router.KeyWpaPsk, f.psk}}
	if f.cipher == router.AES {
		routerConfigParams = append(routerConfigParams, router.AxRouterConfigParam{f.band, router.KeyCrytpo, "aes"})
	} else {
		routerConfigParams = append(routerConfigParams, router.AxRouterConfigParam{f.band, router.KeyCrytpo, "tkip+aes"})
	}
	conf, err := wpa.NewConfigFactory(f.psk, f.opts...).Gen()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't generate inner security config")
	}
	return &AxConfig{routerConfigParams, conf}, nil
}

// NewConfigFactory builds a ConfigFactory.
func NewConfigFactory(band router.BandEnum, psk string, cipher router.CipherEnum, opts ...wpa.Option) *AxConfigFactory {
	return &AxConfigFactory{band, psk, cipher, opts}
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.AxConfigFactory = (*AxConfigFactory)(nil)

func (r *AxConfig) RouterParams() []router.AxRouterConfigParam {
	return r.routerConfigParams
}

func (r *AxConfig) SecConfig() sec.Config {
	return r.secCfg
}
