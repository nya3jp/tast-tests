// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tunneled1x

import (
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"fmt"
)

// Option is the function signature used to specify options of Config.
type Option func(*ConfigFactory)

// OuterProtocol returns an Option which sets the outer (layer1) protocol in Config.
func OuterProtocol(outer string) Option {
	return func(c *ConfigFactory) {
		c.outerProtocol = outer
	}
}

// InnerProtocol returns an Option which sets the inner (layer2) protocol in Config.
func InnerProtocol(inner string) Option {
	return func(c *ConfigFactory) {
		c.innerProtocol = inner
	}
}

// ClientPassword returns an Option which sets the client password in Config.
// Note that this is used for setting a bad password for testing, that is,
// it will be set to be the same as server's by default.
func ClientPassword(passwd string) Option {
	return func(c *ConfigFactory) {
		c.clientPassword = passwd
	}
}

// Options below are re-wrapped from the options of package wpaeap.

// AltSubjectMatch returns an Option which sets shill EAP.SubjectAlternativeNameMatch property in Config.
func AltSubjectMatch(sans []string) Option {
	return func(c *ConfigFactory) {
		c.wpaeapOps = append(c.wpaeapOps, wpaeap.AltSubjectMatch(sans))
	}
}

// FileSuffix returns an Option which sets the file suffix in Config.
func FileSuffix(suffix string) Option {
	return func(c *ConfigFactory) {
		c.wpaeapOps = append(c.wpaeapOps, wpaeap.FileSuffix(suffix))
	}
}

// Mode returns an Option which sets WPA mode in Config.
func Mode(mode wpa.ModeEnum) Option {
	return func(c *ConfigFactory) {
		c.wpaeapOps = append(c.wpaeapOps, wpaeap.Mode(mode))
	}
}

// Phase2User returns an Option which adds additional phase 2 user in Config.
func Phase2User(user string, passwd string, protocol string) Option {
	return func(c *ConfigFactory) {
		line := fmt.Sprintf(`"%s" %s "%s" [2]`, user, protocol, passwd)
		c.phase2Users = append(c.phase2Users, line)
	}
}
