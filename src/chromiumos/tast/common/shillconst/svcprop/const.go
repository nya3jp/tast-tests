// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package svcprop defines the constant keys of Service's properties in shill.
package svcprop

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	Device         = "Device"
	Name           = "Name"
	Type           = "Type"
	IsConnected    = "IsConnected"
	Mode           = "Mode"
	State          = "State"
	StaticIPConfig = "StaticIPConfig"
	Visible        = "Visible"

	// WiFi service property names.
	Passphrase        = "Passphrase"
	SecurityClass     = "SecurityClass"
	SSID              = "SSID"
	WiFiBSSID         = "WiFi.BSSID"
	FTEnabled         = "WiFi.FTEnabled"
	WiFiFrequency     = "WiFi.Frequency"
	WiFiFrequencyList = "WiFi.FrequencyList"
	WiFiHexSSID       = "WiFi.HexSSID"
	WiFiHiddenSSID    = "WiFi.HiddenSSID"
	WiFiPhyMode       = "WiFi.PhyMode"

	// EAP service property names.
	EAPCACertPEM                   = "EAP.CACertPEM"
	EAPMethod                      = "EAP.EAP"
	EAPInnerEAP                    = "EAP.InnerEAP"
	EAPIdentity                    = "EAP.Identity"
	EAPPassword                    = "EAP.Password"
	EAPPin                         = "EAP.PIN"
	EAPCertID                      = "EAP.CertID"
	EAPKeyID                       = "EAP.KeyID"
	EAPKeyMgmt                     = "EAP.KeyMgmt"
	EAPUseSystemCAs                = "EAP.UseSystemCAs"
	EAPSubjectAlternativeNameMatch = "EAP.SubjectAlternativeNameMatch"
)
