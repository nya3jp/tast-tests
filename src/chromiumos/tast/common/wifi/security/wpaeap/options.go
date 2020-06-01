// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpaeap

import (
	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/common/wifi/security/wpa"
)

// Option is the function signature used to specify options of Config.
type Option func(*ConfigFactory)

// Mode returns an Option which sets WPA mode in Config.
func Mode(mode wpa.ModeEnum) Option {
	return func(c *ConfigFactory) {
		c.blueprint.mode = mode
	}
}

// FTMode returns an Option which sets fast transition mode in Config.
func FTMode(ft wpa.FTModeEnum) Option {
	return func(c *ConfigFactory) {
		c.blueprint.ftMode = ft
	}
}

// NotUseSystemCAs returns an Option which sets that we are NOT using system CAs in Config.
func NotUseSystemCAs() Option {
	return func(c *ConfigFactory) {
		c.blueprint.useSystemCAs = false
	}
}

// AltSubjectMatch returns an Option which sets shill EAP.SubjectAlternativeNameMatch property in Config.
func AltSubjectMatch(sans []string) Option {
	return func(c *ConfigFactory) {
		c.blueprint.altSubjectMatch = append([]string(nil), sans...)
	}
}

// Options below are re-wrapped from the options of package eap.

// FileSuffix returns an Option which sets the file suffix in Config.
func FileSuffix(suffix string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.FileSuffix(suffix))
	}
}

// Identity returns an Option which sets the user to authenticate as in Config.
func Identity(id string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.Identity(id))
	}
}

// ServerEAPUsers returns an Option which sets the EAP users for server in Config.
func ServerEAPUsers(users string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ServerEAPUsers(users))
	}
}

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client in Config.
func ClientCACert(caCert string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientCACert(caCert))
	}
}

// ClientCred returns an Option which sets the PEM encoded credentials for client in Config.
func ClientCred(cred certificate.Credential) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientCred(cred))
	}
}

// TPMID returns an Option which sets the identifier for client cert/key in TPM in Config.
func TPMID(id string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.TPMID(id))
	}
}
