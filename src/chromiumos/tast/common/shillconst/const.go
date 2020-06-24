// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shillconst defines the constants of shill service.
// This is defined under common/ as they might be used in both
// local and remote tests.
package shillconst

import "github.com/godbus/dbus"

// Type values defined in dbus-constants.h
// The values are used both for Service type and Technology type.
const (
	TypeEthernet = "ethernet"
	TypeWifi     = "wifi"
	TypeCellular = "cellular"
	TypeVPN      = "vpn"
	TypePPPoE    = "pppoe"
)

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyAddress         = "Address"
	DevicePropertyInterface       = "Interface"
	DevicePropertyType            = "Type"
	DevicePropertySelectedService = "SelectedService"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
	DevicePropertyEapDetected       = "EapAuthenticatorDetected"
	DevicePropertyEapCompleted      = "EapAuthenticationCompleted"

	// WiFi device property names.
	DevicePropertyWiFiBgscanMethod       = "BgscanMethod"
	DevicePropertyMACAddrRandomEnabled   = "MACAddressRandomizationEnabled"
	DevicePropertyMACAddrRandomSupported = "MACAddressRandomizationSupported"
	DevicePropertyScanning               = "Scanning" // Also for cellular.
)

// IPConfig property names.
const (
	IPConfigPropertyAddress                   = "Address"
	IPConfigPropertyNameServers               = "NameServers"
	IPConfigPropertyBroadcast                 = "Broadcast"
	IPConfigPropertyDomainName                = "DomainName"
	IPConfigPropertyGateway                   = "Gateway"
	IPConfigPropertyMethod                    = "Method"
	IPConfigPropertyMtu                       = "Mtu"
	IPConfigPropertyPeerAddress               = "PeerAddress"
	IPConfigPropertyPrefixlen                 = "Prefixlen"
	IPConfigPropertyVendorEncapsulatedOptions = "VendorEncapsulatedOptions"
	IPConfigPropertyWebProxyAutoDiscoveryURL  = "WebProxyAutoDiscoveryUrl"
	IPConfigPropertyiSNSOptionData            = "iSNSOptionData"
)

// Manager property names.
const (
	ManagerPropertyActiveProfile          = "ActiveProfile"
	ManagerPropertyDevices                = "Devices"
	ManagerPropertyEnabledTechnologies    = "EnabledTechnologies"
	ManagerPropertyProfiles               = "Profiles"
	ManagerPropertyProhibitedTechnologies = "ProhibitedTechnologies"
	ManagerPropertyServices               = "Services"
	ManagerPropertyServiceCompleteList    = "ServiceCompleteList"
)

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyDevice         = "Device"
	ServicePropertyName           = "Name"
	ServicePropertyType           = "Type"
	ServicePropertyIsConnected    = "IsConnected"
	ServicePropertyMode           = "Mode"
	ServicePropertyState          = "State"
	ServicePropertyStaticIPConfig = "StaticIPConfig"
	ServicePropertyVisible        = "Visible"

	// WiFi service property names.
	ServicePropertyPassphrase        = "Passphrase"
	ServicePropertySecurityClass     = "SecurityClass"
	ServicePropertySSID              = "SSID"
	ServicePropertyWiFiBSSID         = "WiFi.BSSID"
	ServicePropertyFTEnabled         = "WiFi.FTEnabled"
	ServicePropertyWiFiFrequency     = "WiFi.Frequency"
	ServicePropertyWiFiFrequencyList = "WiFi.FrequencyList"
	ServicePropertyWiFiHexSSID       = "WiFi.HexSSID"
	ServicePropertyWiFiHiddenSSID    = "WiFi.HiddenSSID"
	ServicePropertyWiFiPhyMode       = "WiFi.PhyMode"

	// EAP service property names.
	ServicePropertyEAPCACertPEM                   = "EAP.CACertPEM"
	ServicePropertyEAPMethod                      = "EAP.EAP"
	ServicePropertyEAPInnerEAP                    = "EAP.InnerEAP"
	ServicePropertyEAPIdentity                    = "EAP.Identity"
	ServicePropertyEAPPassword                    = "EAP.Password"
	ServicePropertyEAPPin                         = "EAP.PIN"
	ServicePropertyEAPCertID                      = "EAP.CertID"
	ServicePropertyEAPKeyID                       = "EAP.KeyID"
	ServicePropertyEAPKeyMgmt                     = "EAP.KeyMgmt"
	ServicePropertyEAPUseSystemCAs                = "EAP.UseSystemCAs"
	ServicePropertyEAPSubjectAlternativeNameMatch = "EAP.SubjectAlternativeNameMatch"
)

// Service state values defined in dbus-constants.h
const (
	ServiceStateIdle              = "idle"
	ServiceStateCarrier           = "carrier"
	ServiceStateAssociation       = "association"
	ServiceStateConfiguration     = "configuration"
	ServiceStateReady             = "ready"
	ServiceStatePortal            = "portal"
	ServiceStateNoConnectivity    = "no-connectivity"
	ServiceStateRedirectFound     = "redirect-found"
	ServiceStatePortalSuspected   = "portal-suspected"
	ServiceStateOffline           = "offline"
	ServiceStateOnline            = "online"
	ServiceStateDisconnect        = "disconnecting"
	ServiceStateFailure           = "failure"
	ServiceStateActivationFailure = "activation-failure"
)

// ServiceConnectedStates is a list of service states that are considered connected.
var ServiceConnectedStates = []string{
	ServiceStatePortal,
	ServiceStateNoConnectivity,
	ServiceStateRedirectFound,
	ServiceStatePortalSuspected,
	ServiceStateOnline,
	ServiceStateReady,
}

// Security options defined in dbus-constants.h
const (
	SecurityWPA   = "wpa"
	SecurityWEP   = "wep"
	SecurityRSN   = "rsn"
	Security8021x = "802_1x"
	SecurityPSK   = "psk"
	SecurityNone  = "none"
)

// ServiceKeyMgmtIEEE8021X is a value of EAPKeyMgmt.
const ServiceKeyMgmtIEEE8021X = "IEEE8021X"

const defaultStorageDir = "/var/cache/shill/"

const (
	// DefaultProfileName is the name of default profile.
	DefaultProfileName = "default"
	// DefaultProfileObjectPath is the dbus object path of default profile.
	DefaultProfileObjectPath dbus.ObjectPath = "/profile/" + DefaultProfileName
	// DefaultProfilePath is the path of default profile.
	DefaultProfilePath = defaultStorageDir + DefaultProfileName + ".profile"
)

// Profile property names.
const (
	ProfilePropertyCheckPortalList           = "CheckPortalList"
	ProfilePropertyEntries                   = "Entries"
	ProfilePropertyName                      = "Name"
	ProfilePropertyPortalURL                 = "PortalURL"
	ProfilePropertyPortalCheckInterval       = "PortalCheckInterval"
	ProfilePropertyServices                  = "Services"
	ProfilePropertyUserHash                  = "UserHash"
	ProfilePropertyProhibitedTechnologies    = "ProhibitedTechnologies"
	ProfilePropertyArpGateway                = "ArpGateway"
	ProfilePropertyLinkMonitorTechnologies   = "LinkMonitorTechnologies"
	ProfilePropertyNoAutoConnectTechnologies = "NoAutoConnectTechnologies"
)

// Profile entry property names.
const (
	ProfileEntryPropertyName = "Name"
	ProfileEntryPropertyType = "Type"
)
