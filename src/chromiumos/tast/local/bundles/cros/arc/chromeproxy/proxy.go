// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproxy provides the go binding of chrome.proxy APIs.
// Its detailed API spec is https://developer.chrome.com/extensions/proxy
package chromeproxy

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// Mode is go binding of chrome.proxy.Mode.
type Mode string

const (
	// ModeDirect represents "direct" mode.
	ModeDirect Mode = "direct"
	// ModeAutoDetect represents "auto_detect" mode.
	ModeAutoDetect = "auto_detect"
	// ModePacScript represents "pac_script" mode.
	ModePacScript = "pac_script"
	// ModeFixedServers represents "fixed_servers" mode.
	ModeFixedServers = "fixed_servers"
	// Skip ModeSystem, just because it is not yet used.
)

// ProxyServer is go binding of chrome.proxy.ProxyServer.
type ProxyServer struct {
	// No support for scheme, yet.
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
}

// ProxyRules is go binding of chrome.proxy.ProxyRules.
type ProxyRules struct {
	SingleProxy ProxyServer `json:"singleProxy,omitempty"`
	// No support for proxyForHttp, proxyForHttps, proxyForFtp and fallbackProxy, yet.
	BypassList []string `json:"bypassList,omitempty"`
}

// PacScript is go binding of chrome.proxy.PacScript.
type PacScript struct {
	URL string `json:"url,omitempty"`
	// No support for data and mandatory, yet.
}

// ProxyConfig is go binding of chrome.proxy.ProxyConfig.
type ProxyConfig struct {
	Rules     ProxyRules `json:"rules,omitempty"`
	PacScript PacScript  `json:"pacScript,omitempty"`
	Mode      Mode       `json:"mode,omitempty"`
}

// ProxySettings is go binding of the value to be passed to chrome.proxy.settings.set.
type ProxySettings struct {
	Value ProxyConfig `json:"value,omitempty"`
	// No support for scope, yet. "regular" is used by default.
}

// SetSettings invokes chrome.proxy.settings.set with given settings value on tconn.
func SetSettings(ctx context.Context, tconn *chrome.TestConn, settings ProxySettings) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.proxy.settings.set.bind(chrome.proxy.settings))`, settings)
}
