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
	OpenWifi
	PeapEthernet
)

// SupportedNetworks is the list of configurations to test they are preserved
// after rollback. Test a simple PSK network configuration, wifi PEAP without
// certificates, open wifi, and ethernet PEAP without certificates.
// Only one test ethernet configuration can be supported at once.
// peapEthernetConfig should not interfere with the connection in the lab device
// but remove from SupportedNetworks if after running these tests they lose
// connection.
var SupportedNetworks = []SupportedConfiguration{
	{Config: pskConfig, Type: "PSK"},
	{Config: peapWifiConfig, Type: "wifi PEAP"},
	{Config: openWifiConfig, Type: "wifi open"},
	{Config: peapEthernetConfig, Type: "ethernet PEAP"},
}

// Simple PSK network configuration.
var pskConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: &nc.WiFiConfigProperties{
			Passphrase: "pass,pass,123",
			Ssid:       "MyHomeWiFi",
			Security:   nc.WpaPsk,
			HiddenSsid: nc.Automatic}}}

// PEAP wifi configuration without certificates.
var peapWifiConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: &nc.WiFiConfigProperties{
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

// Open wifi
var openWifiConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: &nc.WiFiConfigProperties{
			Ssid:       "myOpenWifi",
			Security:   nc.None,
			HiddenSsid: nc.Automatic}}}

// PEAP ethernet configuration without certificates.
var peapEthernetConfig = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Ethernet: &nc.EthernetConfigProperties{
			Eap: &nc.EAPConfigProperties{
				AnonymousIdentity:   "anonymous_identity_ethernet",
				Identity:            "userIdentityEthernet",
				Inner:               "Automatic",
				Outer:               "PEAP",
				Password:            "testPassEthernet",
				SaveCredentials:     true,
				ClientCertType:      "None",
				DomainSuffixMatch:   []string{},
				SubjectAltNameMatch: []nc.SubjectAltName{},
				UseSystemCAs:        false,
			},
			Authentication: "8021X"}}}

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
			peapVerification(ctx, peapWifiExp.Config.TypeConfig.Wifi.Eap, nwSet.TypeProperties.Wifi.Eap)
	case OpenWifi:
		openWifiExp := SupportedNetworks[OpenWifi]
		testing.ContextLogf(ctx, "Verifying the preservation of the %s network", openWifiExp.Type)
		nwPreservation = wifiVerification(ctx, openWifiExp.Config.TypeConfig.Wifi, nwSet.TypeProperties.Wifi)
	case PeapEthernet:
		peapEthernetExp := SupportedNetworks[PeapEthernet]
		testing.ContextLogf(ctx, "Verifying the preservation of the %s network", peapEthernetExp.Type)
		nwPreservation = ethernetVerification(ctx, peapEthernetExp.Config.TypeConfig.Ethernet, nwSet.TypeProperties.Ethernet) &&
			peapVerification(ctx, peapEthernetExp.Config.TypeConfig.Ethernet.Eap, nwSet.TypeProperties.Ethernet.Eap)
	default:
		return false, errors.Errorf("invalid ConfigID %d", nwID)
	}

	return nwPreservation, nil
}

// wifiVerification verifies the elements of the wifi configuration that can be
// compared without particular rules. Passphrase and Eap are not included.
func wifiVerification(ctx context.Context, wifiExp *nc.WiFiConfigProperties, wifiSet *nc.ManagedWiFiProperties) bool {
	if wifiSet.Security != wifiExp.Security ||
		wifiSet.Ssid.ActiveValue != wifiExp.Ssid {
		// Log details about set and expected configuration for debugging.
		testing.ContextLogf(ctx, "Wifi set: %+v", wifiSet)
		testing.ContextLogf(ctx, "Wifi.Ssid set: %+v", wifiSet.Ssid.ActiveValue)
		testing.ContextLogf(ctx, "Wifi expected: %+v", wifiExp)
		return false
	}
	return true
}

// wifiVerificationWithPassphrase verifies the configuration of a wifi including
// the Passphrase.
func wifiVerificationWithPassphrase(ctx context.Context, wifiExp *nc.WiFiConfigProperties, wifiSet *nc.ManagedWiFiProperties) bool {
	verification := wifiVerification(ctx, wifiExp, wifiSet)
	// Passphrase is not passed via cros_network_config, instead mojo passes a
	// constant value if a password is configured. Only check for non-empty.
	if wifiSet.Passphrase.ActiveValue == "" {
		testing.ContextLog(ctx, "Wifi passphrase is empty")
		verification = false
	}
	return verification
}

// peapVerification verifies the elements of the supported PEAP configuration.
// It works for both wifi and ethernet configurations.
func peapVerification(ctx context.Context, peapExp *nc.EAPConfigProperties, peapSet *nc.ManagedEAPProperties) bool {
	// Password is not passed via cros_network_config, instead mojo passes a
	// constant value if a password is configured. Only check for non-empty.
	// Only check for non-empty for ClientCertType (see b/227740677).
	// TODO(crisguerrero): Add check of Eap.Inner when b/227605505 is fixed.
	// TODO(crisguerrero): Add check of Eap.ClientCertType when b/227734735 and
	// b/227740677 are fixed.
	if peapSet.AnonymousIdentity.ActiveValue != peapExp.AnonymousIdentity ||
		peapSet.Identity.ActiveValue != peapExp.Identity ||
		peapSet.Outer.ActiveValue != peapExp.Outer ||
		peapSet.Password.ActiveValue == "" ||
		peapSet.SaveCredentials.ActiveValue != peapExp.SaveCredentials ||
		peapSet.UseSystemCAs.ActiveValue != peapExp.UseSystemCAs {
		// Log details about set and expected configuration for debugging.
		testing.ContextLogf(ctx, "PEAP set: %+v", peapSet)
		testing.ContextLogf(ctx, "PEAP.AnonymousIdentity set: %+v", peapSet.AnonymousIdentity.ActiveValue)
		testing.ContextLogf(ctx, "PEAP.Identity set: %+v", peapSet.Identity.ActiveValue)
		testing.ContextLogf(ctx, "PEAP.Outer set: %+v", peapSet.Outer.ActiveValue)
		testing.ContextLogf(ctx, "PEAP.Password set: %+v", peapSet.Password.ActiveValue)
		testing.ContextLogf(ctx, "PEAP.SaveCredentials set: %+v", peapSet.SaveCredentials.ActiveValue)
		testing.ContextLogf(ctx, "PEAP.UseSystemCAs set: %+v", peapSet.UseSystemCAs.ActiveValue)
		testing.ContextLogf(ctx, "PEAP expected: %+v", peapExp)
		return false
	}
	return true
}

// ethernetVerification verifies the elements of the ethernet configuration that
// can be compared without particular rules. Eap is not included.
func ethernetVerification(ctx context.Context, ethernetExp *nc.EthernetConfigProperties, ethernetSet *nc.ManagedEthernetProperties) bool {
	if ethernetSet.Authentication.ActiveValue != ethernetExp.Authentication {
		// Log details about set and expected configuration for debugging.
		testing.ContextLogf(ctx, "Ethernet set: %+v", ethernetSet)
		testing.ContextLogf(ctx, "Ethernet.Authentication set: %+v", ethernetSet.Authentication.ActiveValue)
		testing.ContextLogf(ctx, "Ethernet expected: %+v", ethernetExp)
		return false
	}
	return true
}
