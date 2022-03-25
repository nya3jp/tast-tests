// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shillconst defines the constants of shill service.
// This is defined under common/ as they might be used in both
// local and remote tests.
package shillconst

import (
	"time"

	"github.com/godbus/dbus/v5"
)

// Timeout constants for shill tests.
const (
	DefaultTimeout = 30 * time.Second
)

// ServiceProviderOverridePath isth path of the modb file to override serviceproviders.pbf
const ServiceProviderOverridePath = "/var/cache/shill/serviceproviders-exclusive-override.pbf"

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
	DevicePropertyIPConfigs       = "IPConfigs"
	DevicePropertyPowered         = "Powered"
	DevicePropertyType            = "Type"
	DevicePropertySelectedService = "SelectedService"

	// Cellular device property names.
	DevicePropertyCellularAPNList            = "Cellular.APNList"
	DevicePropertyCellularHomeProvider       = "Cellular.HomeProvider"
	DevicePropertyCellularICCID              = "Cellular.ICCID"
	DevicePropertyCellularIMEI               = "Cellular.IMEI"
	DevicePropertyCellularIMSI               = "Cellular.IMSI"
	DevicePropertyCellularMDN                = "Cellular.MDN"
	DevicePropertyCellularPolicyAllowRoaming = "Cellular.PolicyAllowRoaming"
	DevicePropertyCellularSIMPresent         = "Cellular.SIMPresent"
	DevicePropertyCellularSIMSlotInfo        = "Cellular.SIMSlotInfo"
	DevicePropertyCellularSIMLockStatus      = "Cellular.SIMLockStatus"

	// Keys into the dictionaries exposed as properties
	DevicePropertyCellularSIMLockStatusLockType    = "LockType"
	DevicePropertyCellularSIMLockStatusLockEnabled = "LockEnabled"
	DevicePropertyCellularSIMLockStatusRetriesLeft = "RetriesLeft"

	// Valid values taken by properties exposed by shill.
	DevicePropertyValueSIMLockTypePIN = "sim-pin"
	DevicePropertyValueSIMLockTypePUK = "sim-puk"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
	DevicePropertyEapDetected       = "EapAuthenticatorDetected"
	DevicePropertyEapCompleted      = "EapAuthenticationCompleted"

	// WiFi device property names.
	DevicePropertyLastWakeReason                     = "LastWakeReason"
	DevicePropertyMACAddrRandomEnabled               = "MACAddressRandomizationEnabled"
	DevicePropertyMACAddrRandomSupported             = "MACAddressRandomizationSupported"
	DevicePropertyNetDetectScanPeriodSeconds         = "NetDetectScanPeriodSeconds"
	DevicePropertyPasspointInterworkingSelectEnabled = "PasspointInterworkingSelectEnabled"
	DevicePropertyScanning                           = "Scanning" // Also for cellular.
	DevicePropertyWakeOnWiFiAllowed                  = "WakeOnWiFiAllowed"
	DevicePropertyWakeOnWiFiFeaturesEnabled          = "WakeOnWiFiFeaturesEnabled"
	DevicePropertyWiFiBgscanMethod                   = "BgscanMethod"
	DevicePropertyWiFiBgscanShortInterval            = "BgscanShortInterval"
	DevicePropertyWiFiScanInterval                   = "ScanInterval"
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
	ManagerPropertyScanAllowRoam          = "WiFi.ScanAllowRoam"
	ManagerPropertyDOHProviders           = "DNSProxyDOHProviders"
	ManagerPropertyPortalHTTPSURL         = "PortalHttpsUrl"
)

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyConnectable       = "Connectable"
	ServicePropertyDevice            = "Device"
	ServicePropertyError             = "Error"
	ServicePropertyName              = "Name"
	ServicePropertyType              = "Type"
	ServicePropertyIsConnected       = "IsConnected"
	ServicePropertyMode              = "Mode"
	ServicePropertyState             = "State"
	ServicePropertyStaticIPConfig    = "StaticIPConfig"
	ServicePropertyStrength          = "Strength"
	ServicePropertyVisible           = "Visible"
	ServicePropertyAutoConnect       = "AutoConnect"
	ServicePropertyGUID              = "GUID"
	ServicePropertyProvider          = "Provider"
	ServicePropertyPriority          = "Priority"
	ServicePropertyEphemeralPriority = "EphemeralPriority"
	ServicePropertyCheckPortal       = "CheckPortal"

	// Cellular service property names.
	ServicePropertyCellularEID             = "Cellular.EID"
	ServicePropertyCellularICCID           = "Cellular.ICCID"
	ServicePropertyCellularAllowRoaming    = "Cellular.AllowRoaming"
	ServicePropertyCellularLastGoodAPN     = "Cellular.LastGoodAPN"
	ServicePropertyCellularLastAttachAPN   = "Cellular.LastAttachAPN"
	ServicePropertyCellularRoamingState    = "Cellular.RoamingState"
	ServicePropertyCellularServingOperator = "Cellular.ServingOperator"

	// Keys into the dictionaries exposed as properties for LastAttachAPN and LastGoodAPN
	DevicePropertyCellularAPNInfoApnName   = "apn"
	DevicePropertyCellularAPNInfoApnSource = "apn_source"
	DevicePropertyCellularAPNInfoApnAttach = "attach"
	DevicePropertyCellularAPNInfoApnIPType = "ip_type"

	// WiFi service property names.
	ServicePropertyPassphrase          = "Passphrase"
	ServicePropertySecurity            = "Security"
	ServicePropertySecurityClass       = "SecurityClass"
	ServicePropertySSID                = "SSID"
	ServicePropertyWiFiBSSID           = "WiFi.BSSID"
	ServicePropertyWiFiFrequency       = "WiFi.Frequency"
	ServicePropertyWiFiFrequencyList   = "WiFi.FrequencyList"
	ServicePropertyWiFiHexSSID         = "WiFi.HexSSID"
	ServicePropertyWiFiHiddenSSID      = "WiFi.HiddenSSID"
	ServicePropertyWiFiRandomMACPolicy = "WiFi.RandomMACPolicy"
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
	ServicePropertyEAPDomainSuffixMatch           = "EAP.DomainSuffixMatch"
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

// Portal Detector default values defined in portal_detector.h
const (
	PortalDetectorDefaultCheckPortalList = "ethernet,wifi,cellular"
)

// Security options defined in dbus-constants.h
const (
	SecurityNone               = "none"
	SecurityWEP                = "wep"
	SecurityWPA                = "wpa"
	SecurityWPAWPA2            = "wpa+wpa2"
	SecurityWPAAll             = "wpa-all"
	SecurityWPA2               = "wpa2"
	SecurityWPA2WPA3           = "wpa2+wpa3"
	SecurityWPA3               = "wpa3"
	SecurityWPAEnterprise      = "wpa-ent"
	SecurityWPAWPA2Enterprise  = "wpa+wpa2-ent"
	SecurityWPAAllEnterprise   = "wpa-all-ent"
	SecurityWPA2Enterprise     = "wpa2-ent"
	SecurityWPA2WPA3Enterprise = "wpa2+wpa3-ent"
	SecurityWPA3Enterprise     = "wpa3-ent"
	SecurityClassNone          = "none"
	SecurityClassWEP           = "wep"
	SecurityClassPSK           = "psk"
	SecurityClass8021x         = "802_1x"
)

// MAC randomization policy constants defined in dbus-constants.h
const (
	MacPolicyHardware            = "Hardware"
	MacPolicyFullRandom          = "FullRandom"
	MacPolicyOUIRandom           = "OUIRandom"
	MacPolicyPersistentRandom    = "PersistentRandom"
	MacPolicyNonPersistentRandom = "NonPersistentRandom"
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

// Cellular Operator info values defined in dbus-constants.h
const (
	// OperatorUUIDKey is the unique identifier of the carrier in the shill DB.
	OperatorUUIDKey = "uuid"
	OperatorCode    = "code"
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
//
//	https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
//	platform2/shill/supplicant/wpa_supplicant.cc
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
	// For error org.freedesktop.ModemManager1.Error.MobileEquipment.IncorrectPin.
	ErrorIncorrectPin = "IncorrectPin"
	// For error org.freedesktop.ModemManager1.Error.MobileEquipment.IncorrectPassword.
	ErrorIncorrectPassword = "Incorrect password"
	ErrorPinFailure        = "Failure"
	// For error org.freedesktop.ModemManager1.Error.MobileEquipment.SimPuk.
	ErrorPukRequired = "SIM PUK required"
	ErrorPinBlocked  = "PinBlocked"
)

// Passpoint credentials property names.
const (
	PasspointCredentialsPropertyDomains            = "Domains"
	PasspointCredentialsPropertyRealm              = "Realm"
	PasspointCredentialsPropertyHomeOIs            = "HomeOIs"
	PasspointCredentialsPropertyRequiredHomeOIs    = "RequiredHomeOIs"
	PasspointCredentialsPropertyRoamingConsortia   = "RoamingConsortia"
	PasspointCredentialsPropertyMeteredOverride    = "MeteredOverride"
	PasspointCredentialsPropertyAndroidPackageName = "AndroidPackageName"
)

// Default ICCID when ICCID is unknown. Defined in dbus-constants.h
const (
	UnknownICCID = "unknown-iccid"
)

// Scaled signal quality constant for shill cellular service.
// Set to 10 = less than 2 bars in the UI
const (
	CellularServiceMinSignalStrength = 10
)
