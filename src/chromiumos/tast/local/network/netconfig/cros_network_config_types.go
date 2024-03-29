// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netconfig

// A simplified version of the types in cros_network_config.mojom and
// network_types.mojom to be used in tests. The JSON marshalling comments are
// required for passing structs to javascript.

// There are fields that should not be included in the json object at all (not
// even as an empty object). Setting the optional fields that are of the type
// struct as a pointer allows them to be nullable and to not appear in the json
// object if not provided.

// TODO(b:223867178) Add a function to convert constants to strings to use in logging.

// Types from ip_address.mojom

// IPAddress represents IP address
type IPAddress struct {
	AddressBytes []uint8
}

// Types from network_types.mojom

// NetworkType is the network technology type.
type NetworkType int

// Types of networks. Note that All and Wireless are only used for filtering.
const (
	All NetworkType = iota
	Cellular
	Ethernet
	Mobile
	Tether
	VPN
	Wireless
	WiFi
)

// DeviceStateType : Device / Technology state for devices.
type DeviceStateType int

const (
	// UninitializedDST : The device is available but not yet initialized and can not be enabled.
	UninitializedDST DeviceStateType = iota
	// DisabledDST : The device is initialized but disabled.
	DisabledDST
	// DisablingDST : The device is in the process of disabling. Enable calls may fail until disabling has completed.
	DisablingDST
	// EnablingDST : The device is in the process of enabling. Disable calls may fail until enabling has completed.
	EnablingDST
	// EnabledDST : The device is enabled. Networks can be configured and connected.
	EnabledDST
	// ProhibitedDST : The device is disabled and enabling the device is prohibited by policy.
	ProhibitedDST
	// UnavailableDST : Not used in DeviceStateProperties, but useful when querying by type.
	UnavailableDST
)

// ConnectionStateType : Connection state of visible networks.
type ConnectionStateType int

const (
	// OnlineCST : The network is connected and internet connectivity is available.
	OnlineCST ConnectionStateType = iota
	// ConnectedCST : The network is connected and not in a detected portal state, but internet connectivity may not be available.
	ConnectedCST
	// PortalCST : The network is connected but a portal state was detected. Internet connectivity may be limited. Additional details are in PortalState.
	PortalCST
	// ConnectingCST : The network is in the process of connecting.
	ConnectingCST
	// NotConnectedCST : The network is not connected.
	NotConnectedCST
)

// PortalState : The captive portal state. Provides additional details when the connection state is kPortal.
type PortalState int

const (
	// UnknownPS : The network is not connected or the portal state is not available.
	UnknownPS PortalState = iota
	// OnlinePS : The network is connected and no portal is detected.
	OnlinePS
	// PortalSuspectedPS :A portal is suspected but no redirect was provided.
	PortalSuspectedPS
	// PortalPS : The network is in a portal state with a redirect URL.
	PortalPS
	// ProxyAuthRequiredPS :A proxy requiring authentication is detected.
	ProxyAuthRequiredPS
	// NoInternetPS : The network is connected but no internet is available and no proxy was detected.
	NoInternetPS
)

// ProxyMode is affecting this network. Includes any settings that affect a given network (i.e. global proxy settings are also considered)
type ProxyMode int

const (
	// DirectPM :Direct connection to the network.
	DirectPM ProxyMode = iota
	// AutoDetectPM  :Try to retrieve a PAC script from http://wpad/wpad.dat.
	AutoDetectPM
	// PacScriptPM :Try to retrieve a PAC script from kProxyPacURL.
	PacScriptPM
	// FixedServersPM :Use a specified list of servers.
	FixedServersPM
	// SystemPM :Use the system's proxy settings.
	SystemPM
)

// Types from cros_network_config.mojom

// OncSource : The ONC source for the network configuration, i.e. whether it is stored in the User or Device profile and whether it was configured by policy.
type OncSource int

const (
	// NoneOS : The network is not remembered, or the property is not configurable.
	NoneOS OncSource = iota
	// UserOS : The configuration is saved in the user profile.
	UserOS
	// DeviceOS : The configuration is saved in the device profile.
	DeviceOS
	// UserPolicyOS : The configuration came from a user policy and is saved in the user profile.
	UserPolicyOS
	// DevicePolicyOS : The configuration came from a device policy and is saved in the device profile.
	DevicePolicyOS
)

// SecurityType is the security for WiFi and Ethernet.
type SecurityType int

// Security types.
const (
	None SecurityType = iota
	Wep8021x
	WepPsk
	WpaEap
	WpaPsk
)

// AuthenticationType : The authentication type for Ethernet networks.
type AuthenticationType int

// Ethernet Authenticationtype
const (
	NoneAT AuthenticationType = iota
	K8021x
)

// HiddenSsidMode is the tri-state status of hidden SSID.
type HiddenSsidMode int

// Whether SSID is hidden.
const (
	Automatic HiddenSsidMode = iota
	Disabled
	Enabled
)

// ActivationStateType : Activation state for Cellular networks.
type ActivationStateType int

// ActivationStateType values
const (
	UnknownAST ActivationStateType = iota
	NotActivatedAST
	ActivatingAST
	PartiallyActivatedAST
	ActivatedAST
	// NoServiceAST : A cellular modem exists, but no network service is available.
	NoServiceAST
)

// CellularStateProperties is member of NetworkTypeStateProperties
type CellularStateProperties struct {
	Iccid             string              `json:"iccid"`
	Eid               string              `json:"eid"`
	ActivationState   ActivationStateType `json:"activationSstate"`
	NetworkTechnology string              `json:"networkTechnology"`
	Roaming           bool                `json:"roaming"`
	SignalStrength    int32               `json:"signalStrength"`
	SimLocked         bool                `json:"simLocked"`
}

// WiFiStateProperties is member of NetworkTypeStateProperties
type WiFiStateProperties struct {
	Bssid          string       `json:"bssid"`
	Frequency      int32        `json:"frequency"`
	HexSsid        string       `json:"hexSsid"`
	Security       SecurityType `json:"security"`
	SignalStrength int32        `json:"signalStrength"`
	Ssid           string       `json:"ssid"`
	HiddenSsid     bool         `json:"hiddenSsid"`
}

// EthernetStateProperties is member of NetworkTypeStateProperties
type EthernetStateProperties struct {
	Authentication AuthenticationType `json:"authentication"`
}

// NetworkTypeStateProperties is union which is member of NetworkTypeStateProperties
type NetworkTypeStateProperties struct {
	Cellular CellularStateProperties `json:"cellular,omitempty"`
	Ethernet EthernetStateProperties `json:"ethernet,omitempty"`
	//	Tether   TetherStateProperties   `json:"tether,omitempty"`
	//	VPN      VPNStateProperties      `json:"vpn,omitempty"`
	WiFi WiFiStateProperties `json:"wifi,omitempty"`
}

// NetworkStateProperties is returned by GetNetworkStateList
type NetworkStateProperties struct {
	Connectable        bool                       `json:"connnectable"`
	ConnectRequested   bool                       `json:"connectRequested"`
	ConnectionState    ConnectionStateType        `json:"connectionState"`
	ErrorState         string                     `json:"errorState,omitempty"`
	GUID               string                     `json:"guid"`
	Name               string                     `json:"name"`
	PortalState        PortalState                `json:"portalState"`
	Priority           int32                      `json:"priority"`
	ProxyMode          ProxyMode                  `json:"proxyMode"`
	ProhibitedByPolicy bool                       `json:"prohibitedByPolicy"`
	Source             OncSource                  `json:"source"`
	Type               NetworkType                `json:"type"`
	TypeState          NetworkTypeStateProperties `json:"typeState"`
}

// ManagedString contains active value, if required one may add policy value
// and source.
type ManagedString struct {
	ActiveValue string `json:"activeValue"`
}

// ManagedStringList contains active value, if required one may add policy value
// and source.
type ManagedStringList struct {
	ActiveValue []string `json:"activeValue"`
}

// ManagedBoolean contains active value, if required one may add policy value
// and source.
type ManagedBoolean struct {
	ActiveValue bool `json:"activeValue"`
}

// ManagedSubjectAltNameMatchList contains active value, if required one may add
// policy value and source.
type ManagedSubjectAltNameMatchList struct {
	ActiveValue []SubjectAltName `json:"activeValue"`
}

// ManagedEAPProperties contains properties for EAP networks.
// Currently only PEAP networks without CA certificates are supported.
// We include the same fields as EAPConfigProperties.
type ManagedEAPProperties struct {
	AnonymousIdentity   ManagedString                  `json:"anonymousIdentity,omitempty"`
	Identity            ManagedString                  `json:"identity,omitempty"`
	Inner               ManagedString                  `json:"inner,omitempty"`
	Outer               ManagedString                  `json:"outer,omitempty"`
	Password            ManagedString                  `json:"password,omitempty"`
	SaveCredentials     ManagedBoolean                 `json:"saveCredentials,omitempty"`
	ClientCertType      ManagedString                  `json:"clientCertType,omitempty"`
	DomainSuffixMatch   ManagedStringList              `json:"domainSuffixMatch,omitempty"`
	SubjectAltNameMatch ManagedSubjectAltNameMatchList `json:"subjectAltNameMatch,omitempty"`
	UseSystemCAs        ManagedBoolean                 `json:"useSystemCas,omitempty"`
}

// ManagedWiFiProperties contain managed properties of a wifi connection.
type ManagedWiFiProperties struct {
	// Passphrase is only used for PSK networks and Eap is only used for EAP.
	Eap        *ManagedEAPProperties `json:"eap,omitempty"`
	Passphrase *ManagedString        `json:"passphrase,omitempty"`
	Ssid       ManagedString         `json:"ssid"`
	Security   SecurityType          `json:"security"`
}

// ManagedEthernetProperties contains managed properties of an ethernet
// connection.
type ManagedEthernetProperties struct {
	// Authentication represents the configured authentication type for an
	// Ethernet network.
	Authentication *ManagedString        `json:"authentication,omitempty"`
	Eap            *ManagedEAPProperties `json:"eap,omitempty"`
}

// NetworkTypeManagedProperties contains managed properties for one of the
// network types. Its type is an union, so only one of the fields should be set
// simultaneously.
// Currently only Ethernet and Wifi are implemented.
type NetworkTypeManagedProperties struct {
	Ethernet *ManagedEthernetProperties `json:"ethernet,omitempty"`
	Wifi     *ManagedWiFiProperties     `json:"wifi,omitempty"`
}

// ManagedProperties are provided by GetManagedProperties, see onc_spec.md for
// details.
type ManagedProperties struct {
	Type           NetworkType                  `json:"type"`
	TypeProperties NetworkTypeManagedProperties `json:"typeProperties"`
}

// SubjectAltNameType is the type for SubjectAltName.
type SubjectAltNameType int

// Allowed types for the alternative subject name.
const (
	Email SubjectAltNameType = iota
	DNS
	URI
)

// SubjectAltName contains the information of an alternative subject name.
type SubjectAltName struct {
	Type  SubjectAltNameType `json:"type"`
	Value string             `json:"value"`
}

// EAPConfigProperties contains properties for EAP networks.
// Currently only PEAP networks without CA certificates are supported, so the
// fields related to certificates are not included: ServerCAPEMs, ServerCARefs,
// ServerCARef (deprecated), SubjectMatch, TLSVersionMax, UseProactiveKeyCaching.
type EAPConfigProperties struct {
	AnonymousIdentity   string           `json:"anonymousIdentity,omitempty"`
	Identity            string           `json:"identity,omitempty"`
	Inner               string           `json:"inner,omitempty"` // "Automatic"
	Outer               string           `json:"outer"`           // "PEAP"
	Password            string           `json:"password,omitempty"`
	SaveCredentials     bool             `json:"saveCredentials,omitempty"` // true.
	ClientCertType      string           `json:"clientCertType,omitempty"`  // "None".
	DomainSuffixMatch   []string         `json:"domainSuffixMatch"`         // Empty in manual example but not optional in mojo.
	SubjectAltNameMatch []SubjectAltName `json:"subjectAltNameMatch"`       // Empty in manual example but not optional in mojo.
	UseSystemCAs        bool             `json:"useSystemCAs,omitempty"`    // false. Defaults to true.
}

// WiFiConfigProperties is used to create new configurations or augment
// existing ones.
type WiFiConfigProperties struct {
	// Eap configuration is only used if the wifi security is WpaEap.
	Eap        *EAPConfigProperties `json:"eap,omitempty"`
	Passphrase string               `json:"passphrase,omitempty"`
	Ssid       string               `json:"ssid,omitempty"`
	Security   SecurityType         `json:"security"`
	HiddenSsid HiddenSsidMode       `json:"hiddenSsid"`
}

// EthernetConfigProperties is used to create ethernet configurations.
type EthernetConfigProperties struct {
	// Eap configuration is only used if the ethernet authentication is 8021X.
	Authentication string               `json:"authentication,omitempty"`
	Eap            *EAPConfigProperties `json:"eap,omitempty"`
}

// NetworkTypeConfigProperties contains properties for one type of network. Its
// type is an union, so only one of the fields should be set simultaneously.
// Currently only Ethernet and Wifi are supported.
type NetworkTypeConfigProperties struct {
	Ethernet *EthernetConfigProperties `json:"ethernet,omitempty"`
	VPN      *VPNConfigProperties      `json:"vpn,omitempty"`
	Wifi     *WiFiConfigProperties     `json:"wifi,omitempty"`
}

// ConfigProperties is passed to SetProperties or ConfigureNetwork to configure
// a new network or augment an existing one.
type ConfigProperties struct {
	Name       string                      `json:"name"`
	TypeConfig NetworkTypeConfigProperties `json:"typeConfig"`
}

// FilterType is used for requesting lists of network states.
type FilterType int

const (
	// ActiveFT :Return active networks. A network is active when its ConnectionStateType != kNotConnected.
	ActiveFT FilterType = iota
	// VisibleFT :Return visible (active, physically connected or in-range) networks. Active networks will be listed first.
	VisibleFT
	// ConfiguredFT :Only include configured (saved) networks.
	ConfiguredFT
	// AllFT :Include all networks.
	AllFT
)

// NetworkFilter is passed to GetNetworkStateList to filter the list of networks returned.
type NetworkFilter struct {
	Filter      FilterType  `json:"filter"`
	NetworkType NetworkType `json:"networktype"`
	Limit       int32       `json:"limit"`
}

// SIMLockStatus is the SIM card lock status for Cellular networks.
type SIMLockStatus struct {
	LockType    string `json:"locktype"`
	LockEnabled bool   `json:"lockenabled"`
	RetriesLeft int32  `json:"retriesleft"`
}

// SIMInfo is details about a sim slot available on the device.
type SIMInfo struct {
	SlotID    int32  `json:"slotID"`
	Eid       string `json:"eid"`
	Iccid     string `json:"iccid"`
	IsPrimary bool   `json:"isPrimary"`
}

// InhibitReason : Reasons why the Cellular Device may have its scanning inhibited (i.e. temporarily stopped).
type InhibitReason int

const (
	// NotInhibited :Not inhibited
	NotInhibited InhibitReason = iota
	// InstallingProfile Inhibited because an eSIM profile is being installed.
	InstallingProfile
	// RenamingProfile :Inhibited because an eSIM profile is being renamed.
	RenamingProfile
	// RemovingProfile :Inhibited because an eSIM profile is being removed.
	RemovingProfile
	// ConnectingToProfile :Inhibited because a connection is in progress which requires that the device switch to a different eSIM profile
	ConnectingToProfile
	// RefreshingProfileList :Inhibited because the list of pending eSIM profiles is being refreshed by checking with an SMDS server.
	RefreshingProfileList
	// ResettingEuiccMemory :Inhibited because the EUICC memory is being reset.
	ResettingEuiccMemory
	// DisablingProfile :Inhibited because an eSIM profile is being disabled.
	DisablingProfile
)

// DeviceStateProperties is returned by GetDeviceStateList
type DeviceStateProperties struct {
	Ipv4Address             IPAddress       `json:"ipv4address,omitempty"`
	Ipv6Address             IPAddress       `json:"ipv6address,omitempty"`
	MacAddress              string          `json:"macaddress,omitempty"`
	Scanning                bool            `json:"scanning"`
	SimLockStatus           SIMLockStatus   `json:"simlockstatus"`
	SimInfos                []SIMInfo       `json:"siminfos,omitempty"`
	InhibitReason           InhibitReason   `json:"inhibitreason"`
	SimAbsent               bool            `json:"simabsent"`
	DeviceState             DeviceStateType `json:"devicestate"`
	Type                    NetworkType     `json:"type"`
	ManagedNetworkAvailable bool            `json:"managednetworkavailable"`
}

// VPNConfigProperties is used to create new VPN services or augment existing
// ones.
type VPNConfigProperties struct {
	Host      string                     `json:"host"`
	Type      VPNTypeConfig              `json:"type"`
	IPsec     *IPsecConfigProperties     `json:"ipSec"`
	L2TP      *L2TPConfigProperties      `json:"l2tp"`
	OpenVPN   *OpenVPNConfigProperties   `json:"openVpn"`
	WireGuard *WireGuardConfigProperties `json:"wireguard"`
}

// VPNType is the type of a VPN service.
type VPNType int

// VPN types.
const (
	VPNTypeIKEv2 VPNType = iota
	VPNTypeL2TPIPsec
	VPNTypeOpenVPN
	VPNTypeWireGuard
	VPNTypeExtension
	VPNTypeARC
)

// VPNTypeConfig represents the type of a VPN service.
type VPNTypeConfig struct {
	Value VPNType `json:"value"`
}

// IPsecConfigProperties contains the properties to config IPsec tunnel for a VPN
// service.
type IPsecConfigProperties struct {
	AuthType     string               `json:"authenticationType"`
	EAP          *EAPConfigProperties `json:"eap,omitempty"`
	IKEVersion   int                  `json:"ikeVersion"`
	LocalID      string               `json:"localIdentity"`
	PSK          string               `json:"psk"`
	RemoteID     string               `json:"remoteIdentity"`
	ServerCAPEMs []string             `json:"serverCaPems"`
}

// L2TPConfigProperties contains the properties to config L2TP tunnel for a VPN
// service.
type L2TPConfigProperties struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// OpenVPNConfigProperties contains the properties to config a OpenVPN service.
type OpenVPNConfigProperties struct {
	ClientCertType         string   `json:"clientCertType"`
	ClientCertPkcs11Id     string   `json:"clientCertPkcs11Id"`
	ExtraHosts             []string `json:"extraHosts"`
	Password               string   `json:"password"`
	ServerCAPEMs           []string `json:"serverCaPems"`
	Username               string   `json:"username"`
	UserAuthenticationType string   `json:"userAuthenticationType"`
}

// WireGuardConfigProperties contains the properties to config a WireGuard
// service.
type WireGuardConfigProperties struct {
	PrivateKey *string                   `json:"privateKey,omitempty"`
	Peers      []WireGuardPeerProperties `json:"peers"`
}

// WireGuardPeerProperties contains the properties to config a peer in WireGuard
// services.
type WireGuardPeerProperties struct {
	PublicKey    string  `json:"publicKey"`
	PresharedKey *string `json:"presharedKey,omitempty"`
	AllowedIPs   string  `json:"allowedIps"`
	Endpoint     string  `json:"endpoint"`
	KeepAlive    int     `json:"persistentKeepAlive"`
}
