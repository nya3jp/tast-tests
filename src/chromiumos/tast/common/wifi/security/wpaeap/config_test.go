// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpaeap

import (
	"fmt"
	"reflect"
	"testing"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/common/wifi/security/wpa"
)

func serverCred() certificate.Credential {
	return certificate.Credential{Cert: "ServerCert", PrivateKey: "ServerKey"}
}

func clientCred() certificate.Credential {
	return certificate.Credential{Cert: "ClientCert", PrivateKey: "ClientKey"}
}

// Common settings for the tests below.
const (
	caCert = "ServerCACert"

	fileSuffix        = "FileSuffix"
	identity          = "testing"
	serverEAPUsers    = `"example user" TLS`
	altSubjectMatch   = `{"Type":"DNS","Value":"wrong_dns.com"}`
	domainSuffixMatch = "example.com"

	caCertPath  = "/tmp/server_ca_cert_file"
	certPath    = "/tmp/server_cert_file"
	keyPath     = "/tmp/server_key_file"
	eapUserPath = "/tmp/server_eap_user_cert_file"

	slotID    = 2
	pin       = "77777"
	netCertID = "8888"
)

// TestWPAEAPDefault tests the default behavior of NewConfigFactory.
func TestWPAEAPDefault(t *testing.T) {
	// Calling without option to check default values.
	confInterface, err := NewConfigFactory(caCert, serverCred()).Gen()
	if err != nil {
		t.Fatal("failed to Gen with default values")
	}

	conf := confInterface.(*Config)
	// eap.Config should be tested in eap package.
	conf.Config = nil

	expectedConf := &Config{
		mode:              wpa.ModePureWPA,
		ftMode:            wpa.FTModeNone,
		useSystemCAs:      true,
		altSubjectMatch:   nil,
		domainSuffixMatch: nil,
	}

	if !reflect.DeepEqual(conf, expectedConf) {
		t.Fatalf("got %v, want %v", conf, expectedConf)
	}
}

// TestGen tests the result of ConfigFactory.Gen.
func TestGen(t *testing.T) {
	// Build a expected Config.
	// We need to go through eap.NewConfigFactory and eap.ConfigFactory.Gen, then assign
	// the result to Config, because the fields of eap.Config are not exported.
	expectedEAPConfInterface, err := eap.NewConfigFactory(
		caCert, serverCred(),
		eap.FileSuffix(fileSuffix),
		eap.Identity(identity),
		eap.ServerEAPUsers(serverEAPUsers),
		eap.ClientCACert(caCert),
		eap.ClientCred(clientCred()),
	).Gen()
	if err != nil {
		t.Fatal("falied to Gen eap.Config, there should be a bug in eap package: ")
	}
	expectedConf := &Config{
		Config:            expectedEAPConfInterface.(*eap.Config),
		mode:              wpa.ModePureWPA2,
		ftMode:            wpa.FTModeMixed,
		useSystemCAs:      false,
		altSubjectMatch:   []string{altSubjectMatch},
		domainSuffixMatch: []string{domainSuffixMatch},
	}

	confInterface, err := NewConfigFactory(
		caCert, serverCred(),
		Mode(wpa.ModePureWPA2),
		FTMode(wpa.FTModeMixed),
		NotUseSystemCAs(),
		AltSubjectMatch([]string{altSubjectMatch}),
		DomainSuffixMatch([]string{domainSuffixMatch}),
		FileSuffix(fileSuffix),
		Identity(identity),
		ServerEAPUsers(serverEAPUsers),
		ClientCACert(caCert),
		ClientCred(clientCred()),
	).Gen()
	if err != nil {
		t.Fatal("failed to Gen Config")
	}
	conf := confInterface.(*Config)

	if !reflect.DeepEqual(conf, expectedConf) {
		t.Errorf("got %v, want %v", conf, expectedConf)
	}
}

// TestHostapdConfig tests the result of Config.HostapdConfig.
func TestHostapdConfig(t *testing.T) {
	for _, c := range []struct {
		opts        []Option
		expectProps map[string]string
	}{
		{
			opts: []Option{Mode(wpa.ModePureWPA2)},
			expectProps: map[string]string{
				"wpa_key_mgmt": "WPA-EAP FT-EAP",
			},
		},
		{
			opts: []Option{Mode(wpa.ModePureWPA3)},
			expectProps: map[string]string{
				"wpa_key_mgmt": "WPA-EAP-SHA256 FT-EAP",
			},
		},
	} {
		expectedHostapdConfig := map[string]string{
			"wpa":          "2",
			"wpa_pairwise": "CCMP",

			// Generated by eap.Config.
			"ieee8021x":     "1",
			"eap_server":    "1",
			"ca_cert":       caCertPath,
			"server_cert":   certPath,
			"private_key":   keyPath,
			"eap_user_file": eapUserPath,
		}
		for k, v := range c.expectProps {
			expectedHostapdConfig[k] = v
		}

		commonOpts := []Option{
			FTMode(wpa.FTModeMixed),
			NotUseSystemCAs(),
			AltSubjectMatch([]string{altSubjectMatch}),
			DomainSuffixMatch([]string{domainSuffixMatch}),
			FileSuffix(fileSuffix),
			Identity(identity),
			ServerEAPUsers(serverEAPUsers),
			ClientCACert(caCert),
			ClientCred(clientCred()),
		}
		confInterface, err := NewConfigFactory(
			caCert, serverCred(),
			append(commonOpts, c.opts...)...,
		).Gen()
		if err != nil {
			t.Fatal("failed to Gen Config")
		}
		conf := confInterface.(*Config)

		// Calling HostapdConfig before InstallRouterCredentials should failed.
		if _, err := conf.HostapdConfig(); err == nil {
			t.Error("expect failure due to calling HostapdConfig without InstallRouterCredentials")
		}

		// Normally, these should be generated by conf.InstallRouterCredentials().
		conf.ServerCACertFile = caCertPath
		conf.ServerCertFile = certPath
		conf.ServerKeyFile = keyPath
		conf.ServerEAPUsersFile = eapUserPath

		// Check hostapd config.
		hostapdConfig, err := conf.HostapdConfig()
		if err != nil {
			t.Error("failed to generate hostapd config")
		}
		if !reflect.DeepEqual(hostapdConfig, expectedHostapdConfig) {
			t.Errorf("got %v, want %v", hostapdConfig, expectedHostapdConfig)
		}
	}
}

// TestShillServiceProperties tests the result of Config.ShillServiceProperties.
func TestShillServiceProperties(t *testing.T) {
	expectedShillProperties := map[string]interface{}{
		"EAP.UseSystemCAs":                false,
		"EAP.SubjectAlternativeNameMatch": []string{altSubjectMatch},
		"EAP.DomainSuffixMatch":           []string{domainSuffixMatch},

		// Generated by eap.Config.
		"EAP.Identity":  identity,
		"EAP.PIN":       pin,
		"EAP.CACertPEM": []string{caCert},
		"EAP.CertID":    fmt.Sprintf("%d:%s", slotID, netCertID),
		"EAP.KeyID":     fmt.Sprintf("%d:%s", slotID, netCertID),
	}

	confInterface, err := NewConfigFactory(
		caCert, serverCred(),
		Mode(wpa.ModePureWPA2),
		FTMode(wpa.FTModeMixed),
		NotUseSystemCAs(),
		AltSubjectMatch([]string{altSubjectMatch}),
		DomainSuffixMatch([]string{domainSuffixMatch}),
		FileSuffix(fileSuffix),
		Identity(identity),
		ServerEAPUsers(serverEAPUsers),
		ClientCACert(caCert),
		ClientCred(clientCred()),
	).Gen()
	if err != nil {
		t.Fatal("failed to Gen Config")
	}
	conf := confInterface.(*Config)

	// Calling ShillServiceProperties before InstallClientCredentials should failed.
	if _, err := conf.ShillServiceProperties(); err == nil {
		t.Error("expect failure due to calling ShillServiceProperties without InstallClientCredentials")
	}

	// Normally, these should be generated by conf.InstallClientCredentials().
	conf.ClientSlotID = slotID
	conf.Pin = pin
	conf.NetCertID = netCertID

	// Check shill properties.
	shillProperties, err := conf.ShillServiceProperties()
	if err != nil {
		t.Error("failed to generate shill properties")
	}
	if !reflect.DeepEqual(shillProperties, expectedShillProperties) {
		t.Errorf("got %v, want %v", shillProperties, expectedShillProperties)
	}
}
