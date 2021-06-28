// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shillconst defines the constants of shill service.
// This is defined under common/ as they might be used in both
// local and remote tests.
package shillconst

import (
	"time"

	"github.com/godbus/dbus"
)

// Timeout constants for shill tests.
const (
	DefaultTimeout = 30 * time.Second
)

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
	DevicePropertyDBusObject      = "DBus.Object"
	DevicePropertyInhibited       = "Inhibited"
	DevicePropertyInterface       = "Interface"
	DevicePropertyPowered         = "Powered"
	DevicePropertyType            = "Type"
	DevicePropertySelectedService = "SelectedService"

	// Cellular device property names.
	DevicePropertyCellularICCID       = "Cellular.ICCID"
	DevicePropertyCellularSIMPresent  = "Cellular.SIMPresent"
	DevicePropertyCellularSIMSlotInfo = "Cellular.SIMSlotInfo"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
	DevicePropertyEapDetected       = "EapAuthenticatorDetected"
	DevicePropertyEapCompleted      = "EapAuthenticationCompleted"

	// WiFi device property names.
	DevicePropertyWiFiBgscanMethod           = "BgscanMethod"
	DevicePropertyWiFiScanInterval           = "ScanInterval"
	DevicePropertyWiFiBgscanShortInterval    = "BgscanShortInterval"
	DevicePropertyMACAddrRandomEnabled       = "MACAddressRandomizationEnabled"
	DevicePropertyMACAddrRandomSupported     = "MACAddressRandomizationSupported"
	DevicePropertyScanning                   = "Scanning" // Also for cellular.
	DevicePropertyWakeOnWiFiAllowed          = "WakeOnWiFiAllowed"
	DevicePropertyWakeOnWiFiFeaturesEnabled  = "WakeOnWiFiFeaturesEnabled"
	DevicePropertyLastWakeReason             = "LastWakeReason"
	DevicePropertyNetDetectScanPeriodSeconds = "NetDetectScanPeriodSeconds"
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
	ManagerPropertyAvailableTechnologies  = "AvailableTechnologies"
	ManagerPropertyDevices                = "Devices"
	ManagerPropertyEnabledTechnologies    = "EnabledTechnologies"
	ManagerPropertyProfiles               = "Profiles"
	ManagerPropertyProhibitedTechnologies = "ProhibitedTechnologies"
	ManagerPropertyServices               = "Services"
	ManagerPropertyServiceCompleteList    = "ServiceCompleteList"
	ManagerPropertyGlobalFTEnabled        = "WiFi.GlobalFTEnabled"
)

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyConnectable    = "Connectable"
	ServicePropertyDevice         = "Device"
	ServicePropertyError          = "Error"
	ServicePropertyName           = "Name"
	ServicePropertyType           = "Type"
	ServicePropertyIsConnected    = "IsConnected"
	ServicePropertyMode           = "Mode"
	ServicePropertyState          = "State"
	ServicePropertyStaticIPConfig = "StaticIPConfig"
	ServicePropertyVisible        = "Visible"
	ServicePropertyAutoConnect    = "AutoConnect"
	ServicePropertyGUID           = "GUID"
	ServicePropertyProvider       = "Provider"

	// Cellular service property names.
	ServicePropertyCellularICCID = "Cellular.ICCID"

	// WiFi service property names.
	ServicePropertyPassphrase          = "Passphrase"
	ServicePropertySecurityClass       = "SecurityClass"
	ServicePropertySSID                = "SSID"
	ServicePropertyWiFiBSSID           = "WiFi.BSSID"
	ServicePropertyWiFiFrequency       = "WiFi.Frequency"
	ServicePropertyWiFiFrequencyList   = "WiFi.FrequencyList"
	ServicePropertyWiFiHexSSID         = "WiFi.HexSSID"
	ServicePropertyWiFiHiddenSSID      = "WiFi.HiddenSSID"
	ServicePropertyWiFiPhyMode         = "WiFi.PhyMode"
	ServicePropertyWiFiRekeyInProgress = "WiFi.RekeyInProgress"
	ServicePropertyWiFiRoamState       = "WiFi.RoamState"

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

// Service Error values
const (
	ServiceErrorNoFailure = "no-failure"
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

// Roam state values defined in dbus-constants.h
const (
	RoamStateIdle          = "idle"
	RoamStateAssociation   = "association"
	RoamStateConfiguration = "configuration"
	RoamStateReady         = "ready"
)

// ServiceConnectedStates is a list of service states that are considered connected.
var ServiceConnectedStates = []interface{}{
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
	ProfilePropertyNoAutoConnectTechnologies = "NoAutoConnectTechnologies"
)

// Profile entry property names.
const (
	ProfileEntryPropertyName = "Name"
	ProfileEntryPropertyType = "Type"
)

// The common prefix of DHCP property keys in shill.
const dhcpPropertyPrefix = "DHCPProperty."

// DHCP property names defined in dhcp/dhcp_properties.cc.
// These keys can be used in properties of both Manager or Service.
const (
	DHCPPropertyHostname    = dhcpPropertyPrefix + "Hostname"
	DHCPPropertyVendorClass = dhcpPropertyPrefix + "VendorClass"
)

// Device background scan methods.
// The values are from wpa_supplicant + "none" for no background scan.
// See:
//   https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
//   platform2/shill/supplicant/wpa_supplicant.cc
const (
	DeviceBgscanMethodSimple = "simple"
	DeviceBgscanMethodLearn  = "learn"
	DeviceBgscanMethodNone   = "none"
)

// WakeOnWiFi features.
const (
	WakeOnWiFiFeaturesDarkConnect = "darkconnect"
	WakeOnWiFiFeaturesNone        = "none"
)

// LastWakeReason values.
const (
	WakeOnWiFiReasonDisconnect = "WiFi.Disconnect"
	WakeOnWiFiReasonPattern    = "WiFi.Pattern"
	WakeOnWiFiReasonSSID       = "WiFi.SSID"
	WakeOnWiFiReasonUnknown    = "Unknown"
)

// DBus Errors
const (
	ErrorMatchingServiceNotFound = "Matching service was not found"
	ErrorModemNotStarted         = "Modem not started"
)
