// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rollbacknetworks

import (
	"context"

	"chromiumos/tast/errors"
	nc "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

// SupportedConfiguration contains the configuration of the network and its
// type. Type are for debug purposes and informational logs.
type SupportedConfiguration struct {
	Config nc.ConfigProperties
	Type   string
}

// ConfigID is the index of the configuration in the list of supported networks.
type ConfigID int

// The networks should be in the same order as assigned below.
const (
	Psk ConfigID = iota
	PeapWifi
)

// SupportedNetworks is the list of configurations to test they are preserved
// after rollback. Test a simple PSK network configuration and PEAP without
// certificates.
// TODO(b/227562233): Test all the type of networks that are supported and
// preserved during rollback.
var SupportedNetworks = []SupportedConfiguration{
	{Config: pskConfig, Type: "PSK"},
	{Config: peapWifiConfig, Type: "wifi PEAP"},
}

// Simple PSK network configuration.
var pskConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: nc.WiFiConfigProperties{
			Passphrase: "pass,pass,123",
			Ssid:       "MyHomeWiFi",
			Security:   nc.WpaPsk,
			HiddenSsid: nc.Automatic}}}

// PEAP wifi configuration without certificates.
var peapWifiConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: nc.WiFiConfigProperties{
			Eap: &nc.EAPConfigProperties{
				AnonymousIdentity:   "anonymous_identity",
				Identity:            "userIdentity",
				Inner:               "Automatic",
				Outer:               "PEAP",
				Password:            "testPass",
				SaveCredentials:     true,
				ClientCertType:      "None",
				DomainSuffixMatch:   []string{},
				SubjectAltNameMatch: []nc.SubjectAltName{},
				UseSystemCAs:        false,
			},
			Ssid:       "wifiTestPEAP",
			Security:   nc.WpaEap,
			HiddenSsid: nc.Automatic}}}

// VerifyNetwork checks if the configuration set is the expected one. The
// verification of the fields depends on the configuration set, so the
// appropriate verification methods are called for each of them.
func VerifyNetwork(ctx context.Context, nwID ConfigID, nwSet *nc.ManagedProperties) (bool, error) {
	var nwPreservation bool
	switch nwID {
	case Psk:
		pskExp := SupportedNetworks[Psk]
		testing.ContextLogf(ctx, "Verifying the preservation of the %s network", pskExp.Type)
		nwPreservation = wifiVerificationWithPassphrase(ctx, pskExp.Config.TypeConfig.Wifi, nwSet.TypeProperties.Wifi)
	case PeapWifi:
		peapWifiExp := SupportedNetworks[PeapWifi]
		testing.ContextLogf(ctx, "Verifying the preservation of the %s network", peapWifiExp.Type)
		nwPreservation = wifiVerification(ctx, peapWifiExp.Config.TypeConfig.Wifi, nwSet.TypeProperties.Wifi) &&
			wifiPeapVerification(ctx, peapWifiExp.Config.TypeConfig.Wifi.Eap, nwSet.TypeProperties.Wifi.Eap)
	default:
		return false, errors.Errorf("invalid ConfigID %d", nwID)
	}

	return nwPreservation, nil
}

// wifiVerification verifies the elements of the wifi configuration that can be
// compared without particular rules. Passphrase and Eap are not included.
func wifiVerification(ctx context.Context, wifiExp nc.WiFiConfigProperties, wifiSet nc.ManagedWiFiProperties) bool {
	if wifiSet.Security != wifiExp.Security ||
		wifiSet.Ssid.ActiveValue != wifiExp.Ssid {
		// Log details about set and expected configuration for debugging.
		testing.ContextLogf(ctx, "Wifi set: %+v", wifiSet)
		testing.ContextLogf(ctx, "Wifi expected: %+v", wifiExp)
		return false
	}
	return true
}

// wifiVerificationWithPassphrase verifies the configuration of a wifi including
// the Passphrase.
func wifiVerificationWithPassphrase(ctx context.Context, wifiExp nc.WiFiConfigProperties, wifiSet nc.ManagedWiFiProperties) bool {
	verification := wifiVerification(ctx, wifiExp, wifiSet)
	// Passphrase is not passed via cros_network_config, instead mojo passes a
	// constant value if a password is configured. Only check for non-empty.
	if wifiSet.Passphrase.ActiveValue == "" {
		testing.ContextLog(ctx, "Wifi passphrase is empty")
		verification = false
	}
	return verification
}

// wifiPeapVerification verifies the elements of the supported wifi PEAP
// configuration.
func wifiPeapVerification(ctx context.Context, peapWifiExp *nc.EAPConfigProperties, peapWifiSet *nc.ManagedEAPProperties) bool {
	// Password is not passed via cros_network_config, instead mojo passes a
	// constant value if a password is configured. Only check for non-empty.
	// Only check for non-empty for ClientCertType (see b/227740677).
	// TODO(crisguerrero): Add check of Eap.Inner when b/227605505 is fixed.
	if peapWifiSet.AnonymousIdentity.ActiveValue != peapWifiExp.AnonymousIdentity ||
		peapWifiSet.Identity.ActiveValue != peapWifiExp.Identity ||
		peapWifiSet.Outer.ActiveValue != peapWifiExp.Outer ||
		peapWifiSet.Password.ActiveValue == "" ||
		peapWifiSet.SaveCredentials.ActiveValue != peapWifiExp.SaveCredentials ||
		peapWifiSet.ClientCertType.ActiveValue == "" ||
		peapWifiSet.UseSystemCAs.ActiveValue != peapWifiExp.UseSystemCAs {
		// Log details about set and expected configuration for debugging.
		testing.ContextLogf(ctx, "Wifi PEAP set: %+v", peapWifiSet)
		testing.ContextLogf(ctx, "Wifi PEAP expected: %+v", peapWifiExp)
		return false
	}
	return true
}
