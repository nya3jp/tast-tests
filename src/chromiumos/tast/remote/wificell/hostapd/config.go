// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
)

// ModeEnum is the type for specifying hostap mode.
type ModeEnum string

// Mode enums.
const (
	Mode80211a       ModeEnum = "a"
	Mode80211b       ModeEnum = "b"
	Mode80211g       ModeEnum = "g"
	Mode80211nMixed  ModeEnum = "n-mixed"
	Mode80211nPure   ModeEnum = "n-only"
	Mode80211acMixed ModeEnum = "ac-mixed"
	Mode80211acPure  ModeEnum = "ac-only"
)

// HTCap is the type for specifying HT capabilities in hostapd config (ht_capab=).
type HTCap int

// HTCap enums, use bitmask for ease of checking existence.
const (
	HTCapHT20      HTCap = 1 << iota // HTCaps string "" means HT20.
	HTCapHT40                        // auto-detect supported "[HT40-]" or "[HT40+]"
	HTCapHT40Minus                   // "[HT40-]"
	HTCapHT40Plus                    // "[HT40+]"
	HTCapSGI20                       // "[SHORT-GI-20]"
	HTCapSGI40                       // "[SHORT-GI-40]"
	// The test APs don't support Greenfield now. Comment out the option to avoid usage.
	// (The capability can be shown with `iw phy`)
	// HTCapGreenfield                   // "[GF]"
)

// VHTCap is the type for specifying VHT capabilities in hostapd config (vht_capab=).
type VHTCap string

// Each capability can be simply mapped to a string.
const (
	VHTCapVHT160             VHTCap = "[VHT160]"
	VHTCapVHT16080Plus80     VHTCap = "[VHT160-80PLUS80]"
	VHTCapRXLDPC             VHTCap = "[RXLDPC]"
	VHTCapSGI80              VHTCap = "[SHORT-GI-80]"
	VHTCapSGI160             VHTCap = "[SHORT-GI-160]"
	VHTCapTxSTBC2BY1         VHTCap = "[TX-STBC-2BY1]"
	VHTCapRxSTBC1            VHTCap = "[RX-STBC-1]"
	VHTCapRxSTBC12           VHTCap = "[RX-STBC-12]"
	VHTCapRxSTBC123          VHTCap = "[RX-STBC-123]"
	VHTCapRxSTBC1234         VHTCap = "[RX-STBC-1234]"
	VHTCapSUBeamformer       VHTCap = "[SU-BEAMFORMER]"
	VHTCapSUBeamformee       VHTCap = "[SU-BEAMFORMEE]"
	VHTCapBFAntenna2         VHTCap = "[BF-ANTENNA-2]"
	VHTCapSoundingDimension2 VHTCap = "[SOUNDING-DIMENSION-2]"
	VHTCapMUBeamformer       VHTCap = "[MU-BEAMFORMER]"
	VHTCapMUBeamformee       VHTCap = "[MU-BEAMFORMEE]"
	VHTCapVHTTXOPPS          VHTCap = "[VHT-TXOP-PS]"
	VHTCapHTCVHT             VHTCap = "[HTC-VHT]"
	VHTCapMaxAMPDULenExp0    VHTCap = "[MAX-A-MPDU-LEN-EXP0]"
	VHTCapMaxAMPDULenExp1    VHTCap = "[MAX-A-MPDU-LEN-EXP1]"
	VHTCapMaxAMPDULenExp2    VHTCap = "[MAX-A-MPDU-LEN-EXP2]"
	VHTCapMaxAMPDULenExp3    VHTCap = "[MAX-A-MPDU-LEN-EXP3]"
	VHTCapMaxAMPDULenExp4    VHTCap = "[MAX-A-MPDU-LEN-EXP4]"
	VHTCapMaxAMPDULenExp5    VHTCap = "[MAX-A-MPDU-LEN-EXP5]"
	VHTCapMaxAMPDULenExp6    VHTCap = "[MAX-A-MPDU-LEN-EXP6]"
	VHTCapMaxAMPDULenExp7    VHTCap = "[MAX-A-MPDU-LEN-EXP7]"
	VHTCapVHTLinkADAPT2      VHTCap = "[VHT-LINK-ADAPT2]"
	VHTCapVHTLinkADAPT3      VHTCap = "[VHT-LINK-ADAPT3]"
	VHTCapRxAntennaPattern   VHTCap = "[RX-ANTENNA-PATTERN]"
	VHTCapTxAntennaPattern   VHTCap = "[TX-ANTENNA-PATTERN]"
)

// VHTChWidthEnum is the type for specifying operating channel width in hostapd config (vht_oper_chwidth=).
type VHTChWidthEnum int

// VHTChWidth enums.
const (
	// VHTChWidth20Or40 is the default value when none of VHTChWidth* specified.
	VHTChWidth20Or40 VHTChWidthEnum = iota
	VHTChWidth80
	VHTChWidth160
	VHTChWidth80Plus80
)

// PMFEnum is the type for specifying the setting of "Protected Management Frames" (IEEE802.11w).
type PMFEnum int

// PMF enums.
const (
	PMFDisabled PMFEnum = iota
	PMFOptional
	PMFRequired
)

// AdditionalBSS is the type for specifying parameters of additional BSSs to be advertised on the same phy.
// All fields are required, and must be distinct from the corresponding fields of the primary network.
type AdditionalBSS struct {
	IfaceName string
	SSID      string
	BSSID     string
}

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// SSID returns an Option which sets ssid in hostapd config.
func SSID(ssid string) Option {
	return func(c *Config) {
		c.SSID = ssid
	}
}

// Mode returns an Option which sets mode in hostapd config.
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		c.Mode = mode
	}
}

// Channel returns an Option which sets channel in hostapd config.
func Channel(ch int) Option {
	return func(c *Config) {
		c.Channel = ch
	}
}

// HTCaps returns an Option which sets HT capabilities in hostapd config.
func HTCaps(caps ...HTCap) Option {
	return func(c *Config) {
		for _, ca := range caps {
			c.HTCaps |= ca
		}
	}
}

// VHTCaps returns an Option which sets VHT capabilities in hostapd config.
func VHTCaps(caps ...VHTCap) Option {
	return func(c *Config) {
		c.VHTCaps = append(c.VHTCaps, caps...)
	}
}

// VHTCenterChannel returns an Option which sets VHT center channel in hostapd config.
func VHTCenterChannel(ch int) Option {
	return func(c *Config) {
		c.VHTCenterChannel = ch
	}
}

// VHTChWidth returns an Option which sets VHT operating channel width in hostapd config.
func VHTChWidth(chw VHTChWidthEnum) Option {
	return func(c *Config) {
		c.VHTChWidth = chw
	}
}

// Hidden returns an Option which sets that it is a hidden network in hostapd config.
func Hidden() Option {
	return func(c *Config) {
		c.Hidden = true
	}
}

// SecurityConfig returns an Option which sets the security config in hostapd config.
func SecurityConfig(conf security.Config) Option {
	return func(c *Config) {
		c.SecurityConfig = conf
	}
}

// PMF returns an Options which sets whether protected management frame
// is enabled or required.
func PMF(p PMFEnum) Option {
	return func(c *Config) {
		c.PMF = p
	}
}

// SpectrumManagement returns an Option which enables spectrum management in hostapd config.
func SpectrumManagement() Option {
	return func(c *Config) {
		c.SpectrumManagement = true
	}
}

// DTIMPeriod returns an Option which sets the DTIM period in hostapd config.
func DTIMPeriod(period int) Option {
	return func(c *Config) {
		c.DTIMPeriod = period
	}
}

// BeaconInterval returns an Option which sets the beacon interval in hostapd config.
// The unit is 1kus = 1.024ms. The value should be in 15..65535.
func BeaconInterval(bi int) Option {
	return func(c *Config) {
		c.BeaconInterval = bi
	}
}

// BSSID returns an Option which sets bssid in hostapd config.
func BSSID(bssid string) Option {
	return func(c *Config) {
		c.BSSID = bssid
	}
}

// OBSSInterval returns an Option which sets the interval in seconds between
// overlapping BSS scans. Default value is 0 (disabled).
func OBSSInterval(interval uint16) Option {
	return func(c *Config) {
		c.OBSSInterval = interval
	}
}

// Bridge returns an Option which sets bridge in hostapd config.
func Bridge(br string) Option {
	return func(c *Config) {
		c.Bridge = br
	}
}

// MobilityDomain returns an Option which sets mobility domain in hostapd config.
func MobilityDomain(mdID string) Option {
	return func(c *Config) {
		c.MobilityDomain = mdID
	}
}

// NASIdentifier returns an Option which sets nas_identifier in hostapd config.
func NASIdentifier(id string) Option {
	return func(c *Config) {
		c.NASIdentifier = id
	}
}

// R1KeyHolder returns an Option which sets r1 key holder identifier in hostapd config.
func R1KeyHolder(r1khID string) Option {
	return func(c *Config) {
		c.R1KeyHolder = r1khID
	}
}

// R0KHs returns an Option which sets R0KHs in hostapd config. Each R0KH
// should be in format: <MAC address> <NAS Identifier> <256-bit key as hex string>
func R0KHs(r0KHs ...string) Option {
	return func(c *Config) {
		c.R0KHs = append([]string(nil), r0KHs...)
	}
}

// R1KHs returns an Option which sets R1KHs in hostapd config. Each R1KH
// should be in format: <MAC address> <R1KH-ID> <256-bit key as hex string>
func R1KHs(r1KHs ...string) Option {
	return func(c *Config) {
		c.R1KHs = append([]string(nil), r1KHs...)
	}
}

// MBO returns an Option which enables MBO in hostapd config.
func MBO() Option {
	return func(c *Config) {
		c.MBO = true
	}
}

// RRMBeaconReport returns an Option which enables RRM Beacon Report in hostapd config.
func RRMBeaconReport() Option {
	return func(c *Config) {
		c.RRMBeaconReport = true
	}
}

// AdditionalBSSs returns an Option which sets AdditionalBSSs in hostapd config.
// Each AdditionalBSS should have a unique interface name, SSID, and BSSID. The
// number of AdditionalBSSs is limited by the phy. See the 'valid interface
// combinations' section of `iw phy` for more.
func AdditionalBSSs(bssids ...AdditionalBSS) Option {
	return func(c *Config) {
		c.AdditionalBSSs = append([]AdditionalBSS(nil), bssids...)
	}
}

// SupportedRates returns an Option which sets the supported rates in hostapd config.
func SupportedRates(r ...float32) Option {
	return func(c *Config) {
		c.SupportedRates = append([]float32(nil), r...)
	}
}

// BasicRates returns an Option which sets the basic rates in hostapd config.
func BasicRates(r ...float32) Option {
	return func(c *Config) {
		c.BasicRates = append([]float32(nil), r...)
	}
}

// NewConfig creates a Config with given options.
// Default value of Ssid is a random generated string with prefix "TAST_TEST_" and total length 30.
func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		SSID:           RandomSSID("TAST_TEST_"),
		SecurityConfig: &base.Config{},
	}
	for _, op := range ops {
		op(conf)
	}

	// Assign a random BSSID if not assigned to avoid a known issue of
	// reused BSSID in ath10k driver. For the BSSID re-used cases,
	// we'll have a specific test for that.
	if conf.BSSID == "" {
		randBSSID, err := RandomMAC()
		if err != nil {
			return nil, err
		}
		conf.BSSID = randBSSID.String()
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return conf, nil
}

// Config is the configuration to start hostapd on a router.
type Config struct {
	SSID               string
	Mode               ModeEnum
	Channel            int
	HTCaps             HTCap
	VHTCaps            []VHTCap
	VHTCenterChannel   int
	VHTChWidth         VHTChWidthEnum
	Hidden             bool
	SpectrumManagement bool
	BeaconInterval     int
	SecurityConfig     security.Config
	PMF                PMFEnum
	DTIMPeriod         int
	BSSID              string
	OBSSInterval       uint16
	Bridge             string
	MobilityDomain     string
	NASIdentifier      string
	R1KeyHolder        string
	R0KHs              []string
	R1KHs              []string
	MBO                bool
	RRMBeaconReport    bool
	AdditionalBSSs     []AdditionalBSS
	SupportedRates     []float32
	BasicRates         []float32
}

// Format composes a hostapd.conf based on the given Config, iface and ctrlPath.
// iface is the network interface for the hostapd to run. ctrlPath is the control
// file path for hostapd to communicate with hostapd_cli.
func (c *Config) Format(iface, ctrlPath string) (string, error) {
	var builder strings.Builder
	configure := func(k, v string) {
		fmt.Fprintf(&builder, "%s=%s\n", k, v)
	}

	configure("logger_syslog", "-1")
	configure("logger_syslog_level", "0")
	// Default RTS and frag threshold to "off".
	configure("rts_threshold", "-1")
	configure("fragm_threshold", "2346")
	configure("driver", "nl80211")

	// Configurable.
	configure("ctrl_interface", ctrlPath)
	// ssid2 for printf-escaped string, cf. https://w1.fi/cgit/hostap/plain/hostapd/hostapd.conf
	configure("ssid2", encodeSSID(c.SSID))
	configure("interface", iface)
	configure("channel", strconv.Itoa(c.Channel))

	hwMode, err := c.hwMode()
	if err != nil {
		return "", err
	}
	configure("hw_mode", hwMode)

	if c.is80211n() || c.is80211ac() {
		configure("ieee80211n", "1")
		configure("ht_capab", c.htCapsString())
		if c.Mode == Mode80211nPure {
			configure("require_ht", "1")
		}
	}
	if c.is80211ac() {
		configure("ieee80211ac", "1")
		configure("vht_oper_chwidth", strconv.Itoa(int(c.VHTChWidth)))
		// If not set, ignore this field and use hostapd's default value.
		if c.VHTCenterChannel != 0 {
			configure("vht_oper_centr_freq_seg0_idx", strconv.Itoa(c.VHTCenterChannel))
		}
		configure("vht_capab", c.vhtCapsString())
		if c.Mode == Mode80211acPure {
			configure("require_vht", "1")
		}
	}
	if c.HTCaps != 0 {
		configure("wmm_enabled", "1")
	}
	if c.Hidden {
		configure("ignore_broadcast_ssid", "1")
	}
	if c.SpectrumManagement {
		configure("country_code", "US")          // Required for ieee80211d
		configure("ieee80211d", "1")             // Required for local_pwr_constraint
		configure("local_pwr_constraint", "0")   // No local constraint
		configure("spectrum_mgmt_required", "1") // Requires local_pwr_constraint
	}
	if c.BeaconInterval != 0 {
		configure("beacon_int", strconv.Itoa(c.BeaconInterval))
	}

	if c.DTIMPeriod != 0 {
		configure("dtim_period", strconv.Itoa(c.DTIMPeriod))
	}

	if c.BSSID != "" {
		configure("bssid", c.BSSID)
	}

	if c.OBSSInterval != 0 {
		configure("obss_interval", strconv.FormatUint(uint64(c.OBSSInterval), 10))
	}

	securityConf, err := c.SecurityConfig.HostapdConfig()
	if err != nil {
		return "", err
	}
	for k, v := range securityConf {
		configure(k, v)
	}

	configure("ieee80211w", strconv.Itoa(int(c.PMF)))

	if c.Bridge != "" {
		configure("bridge", c.Bridge)
	}

	if c.MobilityDomain != "" {
		configure("mobility_domain", c.MobilityDomain)
	}
	if c.NASIdentifier != "" {
		configure("nas_identifier", c.NASIdentifier)
	}
	if c.R1KeyHolder != "" {
		configure("r1_key_holder", c.R1KeyHolder)
	}
	if len(c.R0KHs) != 0 {
		for _, r := range c.R0KHs {
			configure("r0kh", r)
		}
	}
	if len(c.R1KHs) != 0 {
		for _, r := range c.R1KHs {
			configure("r1kh", r)
		}
	}

	if c.MBO {
		configure("mbo", "1")
	}

	if c.RRMBeaconReport {
		configure("rrm_beacon_report", "1")
	}

	for _, bssid := range c.AdditionalBSSs {
		if bssid.IfaceName == iface {
			return "", errors.Errorf("found a duplicate interface name: %s", iface)
		}
		configure("bss", bssid.IfaceName)
		configure("ssid", bssid.SSID)
		configure("bssid", bssid.BSSID)
	}

	if len(c.SupportedRates) != 0 {
		// Convert from Mbps to 100Kbps.
		rates := make([]string, len(c.SupportedRates))
		for i, r := range c.SupportedRates {
			rates[i] = fmt.Sprintf("%d", int(r*10))
		}
		configure("supported_rates", strings.Join(rates, " "))
	}

	if len(c.BasicRates) != 0 {
		// Convert from Mbps to 100Kbps.
		rates := make([]string, len(c.BasicRates))
		for i, r := range c.BasicRates {
			rates[i] = fmt.Sprintf("%d", int(r*10))
		}
		configure("basic_rates", strings.Join(rates, " "))
	}

	return builder.String(), nil
}

// PcapFreqOptions returns the options for the caller to set frequency with iw for
// preparing interface for packet capturing.
func (c *Config) PcapFreqOptions() ([]iw.SetFreqOption, error) {
	if c.is80211ac() {
		switch c.VHTChWidth {
		case VHTChWidth80:
			return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidth80)}, nil
		case VHTChWidth160:
			return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidth160)}, nil
		case VHTChWidth80Plus80:
			return nil, errors.New("unsupported 80+80 channel width")
		}
		// fallthrough VHTChWidth20Or40.
	}
	if c.is80211n() || c.is80211ac() {
		// 80211n or 80211ac with VHTChWidth20Or40.
		ht := c.htMode()
		switch ht {
		case HTCapHT40Minus:
			return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Minus)}, nil
		case HTCapHT40Plus:
			return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)}, nil
		default:
			return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT20)}, nil
		}
	}
	return []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthNOHT)}, nil
}

// PerfDesc returns the description of this config.
// Useful for reporting perf metrics.
func (c *Config) PerfDesc() string {
	var mode, width string
	if c.is80211ac() {
		mode = "VHT"
		switch c.VHTChWidth {
		case VHTChWidth80:
			width = "80"
		case VHTChWidth160:
			width = "160"
		case VHTChWidth80Plus80:
			width = "80+80"
		default:
			width = "40"
		}
	} else if c.is80211n() {
		mode = "HT"
		switch c.htMode() {
		case HTCapHT40Minus:
			width = "40m"
		case HTCapHT40Plus:
			width = "40p"
		default:
			width = "20"
		}
	} else {
		mode = "11" + string(c.Mode)
	}
	return fmt.Sprintf("ch%03d_mode%s%s_%s", c.Channel, mode, width, c.SecurityConfig.Class())
}

// validate validates the Config, c.
func (c *Config) validate() error {
	if c.SSID == "" || len(c.SSID) > 32 {
		return errors.New("invalid SSID")
	}
	if c.BSSID != "" && len(c.BSSID) != 17 {
		return errors.New("invalid BSSID")
	}
	if c.Mode == "" {
		return errors.New("invalid mode")
	}
	if c.HTCaps > 0 && !c.is80211n() && !c.is80211ac() {
		return errors.Errorf("HTCap is not supported by mode %s", c.Mode)
	}
	if c.HTCaps == 0 && (c.is80211n() || c.is80211ac()) {
		return errors.New("HTCap should be set in mode 802.11n or 802.11ac")
	}
	if !c.is80211ac() {
		if len(c.VHTCaps) != 0 {
			return errors.Errorf("VHTCap is not supported by mode %s", c.Mode)
		}
		if c.VHTCenterChannel != 0 {
			return errors.Errorf("VHTCenterChannel is not supported by mode %s", c.Mode)
		}
		if c.VHTChWidth != VHTChWidth20Or40 {
			return errors.Errorf("VHTChWidth is not supported by mode %s", c.Mode)
		}
	} else if err := c.validateVHTChWidth(); err != nil {
		return err
	}
	if err := c.validateChannel(); err != nil {
		return err
	}
	if c.BeaconInterval != 0 && (c.BeaconInterval > 65535 || c.BeaconInterval < 15) {
		return errors.Errorf("invalid beacon interval setting %d", c.BeaconInterval)
	}
	if c.SecurityConfig == nil {
		return errors.New("no SecurityConfig set")
	}
	if err := c.validatePMF(); err != nil {
		return err
	}

	if c.DTIMPeriod != 0 {
		if c.DTIMPeriod > 255 || c.DTIMPeriod < 1 {
			return errors.Errorf("invalid DTIM period: got %d; want [1..255]", c.DTIMPeriod)
		}
	}

	if c.MobilityDomain != "" {
		if b, err := hex.DecodeString(c.MobilityDomain); err != nil {
			return errors.Wrap(err, "mobility domain should be a hex string")
		} else if len(b) != 2 {
			return errors.New("mobility domain should be 2-octet identifier as a hex string")
		}
	}

	if c.R1KeyHolder != "" {
		if b, err := hex.DecodeString(c.R1KeyHolder); err != nil {
			return errors.Wrap(err, "r1 key holder identifier should be a hex string")
		} else if len(b) != 6 {
			return errors.New("r1 key holder identifier should be 6-octet identifier as a hex string")
		}
	}

	for i, khs := range append(c.R0KHs, c.R1KHs...) {
		fields := strings.Fields(khs)
		if len(fields) != 3 {
			return errors.Errorf("key holder should have exactly three fields, got %q", khs)
		}
		if _, err := net.ParseMAC(fields[0]); err != nil {
			return errors.Wrap(err, "the first field of key holders should be a MAC address")
		}
		if i >= len(c.R0KHs) {
			if _, err := net.ParseMAC(fields[1]); err != nil {
				return errors.Wrap(err, "the second field of r1kh should be a MAC address")
			}
		}
		if b, err := hex.DecodeString(fields[2]); err != nil {
			return errors.Wrap(err, "the third field of key holders should be a hex string")
		} else if len(b) != 32 {
			return errors.Errorf("the third field of key holders should be a 256-bits hex string, got len=%d", len(b))
		}
	}

	ifaces := map[string]struct{}{}
	ssids := map[string]struct{}{c.SSID: {}}
	bssids := map[string]struct{}{c.BSSID: {}}
	for _, bssid := range c.AdditionalBSSs {
		if _, ok := ifaces[bssid.IfaceName]; ok {
			return errors.Errorf("duplicate interface name %s in additional BSSIDs", bssid.IfaceName)
		}
		ifaces[bssid.IfaceName] = struct{}{}
		if _, ok := ssids[bssid.SSID]; ok {
			return errors.Errorf("duplicate SSID %s in additional BSSIDs", bssid.SSID)
		}
		ssids[bssid.SSID] = struct{}{}
		if _, ok := bssids[bssid.BSSID]; ok {
			return errors.Errorf("duplicate BSSID %s in additional BSSIDs", bssid.BSSID)
		}
		bssids[bssid.BSSID] = struct{}{}
	}

	return nil
}

// Helpers for Config to validate.

func channelIn(ch int, list []int) bool {
	for _, c := range list {
		if c == ch {
			return true
		}
	}
	return false
}

var ht40MinusChannels = []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 40, 48, 56, 64, 104, 112, 120, 128, 136, 144, 153, 161}
var ht40PlusChannels = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 36, 44, 52, 60, 100, 108, 116, 124, 132, 140, 149, 157}

func supportHT40Plus(ch int) bool {
	return channelIn(ch, ht40PlusChannels)
}

func supportHT40Minus(ch int) bool {
	return channelIn(ch, ht40MinusChannels)
}

func (c *Config) validateChannel() error {
	f, err := ChannelToFrequency(c.Channel)
	if err != nil {
		return errors.New("invalid channel")
	}

	modeErr := errors.Errorf("mode %s does not support ch%d", c.Mode, c.Channel)
	switch c.Mode {
	case Mode80211a:
		if f < 5000 {
			return modeErr
		}
	case Mode80211b, Mode80211g:
		if f > 5000 {
			return modeErr
		}
	}

	htPlus := supportHT40Plus(c.Channel)
	htMinus := supportHT40Minus(c.Channel)
	if c.HTCaps&HTCapHT40 > 0 && !htPlus && !htMinus {
		return errors.Errorf("ch%d does not support HTCap40", c.Channel)
	}
	if c.HTCaps&HTCapHT40Plus > 0 && !htPlus {
		return errors.Errorf("ch%d does not support HT40+", c.Channel)
	}
	if c.HTCaps&HTCapHT40Minus > 0 && !htMinus {
		return errors.Errorf("ch%d does not support HT40-", c.Channel)
	}
	return nil
}

// Helpers for Config to generate config map.

func (c *Config) is80211n() bool {
	return c.Mode == Mode80211nMixed || c.Mode == Mode80211nPure
}

func (c *Config) is80211ac() bool {
	return c.Mode == Mode80211acMixed || c.Mode == Mode80211acPure
}

func (c *Config) hwMode() (string, error) {
	if c.Mode == Mode80211a || c.Mode == Mode80211b || c.Mode == Mode80211g {
		return string(c.Mode), nil
	}
	if c.is80211n() || c.is80211ac() {
		f, err := ChannelToFrequency(c.Channel)
		if err != nil {
			return "", err
		}
		if f > 5000 {
			return string(Mode80211a), nil
		}
		return string(Mode80211g), nil
	}
	return "", errors.Errorf("invalid mode %s", string(c.Mode))
}

// htMode returns which of HT20, HT40+ and HT40- is used or 0 otherwise.
func (c *Config) htMode() HTCap {
	if c.HTCaps&(HTCapHT40|HTCapHT40Minus) > 0 && supportHT40Minus(c.Channel) {
		return HTCapHT40Minus
	}
	if c.HTCaps&(HTCapHT40|HTCapHT40Plus) > 0 && supportHT40Plus(c.Channel) {
		return HTCapHT40Plus
	}
	if c.HTCaps&HTCapHT20 > 0 {
		return HTCapHT20
	}
	return 0
}

func (c *Config) htCapsString() string {
	var caps []string
	htMode := c.htMode()
	switch htMode {
	case HTCapHT40Minus:
		caps = append(caps, "[HT40-]")
	case HTCapHT40Plus:
		caps = append(caps, "[HT40+]")
	default:
		// HT20 is default and no config string needed.
	}
	if c.HTCaps&HTCapSGI20 > 0 {
		caps = append(caps, "[SHORT-GI-20]")
	}
	if c.HTCaps&HTCapSGI40 > 0 {
		caps = append(caps, "[SHORT-GI-40]")
	}
	return strings.Join(caps, "")
}

func (c *Config) vhtCapsString() string {
	caps := make([]string, len(c.VHTCaps))
	for i, v := range c.VHTCaps {
		caps[i] = string(v)
	}
	return strings.Join(caps, "")
}

func (c *Config) validateVHTChWidth() error {
	switch c.VHTChWidth {
	case VHTChWidth20Or40, VHTChWidth80, VHTChWidth160, VHTChWidth80Plus80:
		return nil
	default:
		return errors.Errorf("invalid vht_oper_chwidth %d", int(c.VHTChWidth))
	}
}

func (c *Config) validatePMF() error {
	switch c.PMF {
	case PMFDisabled:
		return nil
	case PMFOptional, PMFRequired:
		secClass := c.SecurityConfig.Class()
		if secClass == shillconst.SecurityNone || secClass == shillconst.SecurityWEP {
			return errors.Errorf("class %s does not support PMF", secClass)
		}
		return nil
	default:
		return errors.Errorf("invalid PMFEnum %d", int(c.PMF))
	}
}

// encodeSSID encodes ssid into the format that hostapd can read.
// The "%q" format in golang does not work for the case as it contains more
// escape sequence than what printf_decode in hostapd can understand.
// Duplicate the logic of printf_encode in hostapd here.
func encodeSSID(s string) string {
	var builder strings.Builder

	// Always start with 'P"' prefix as printf-encoded format in hostapd.
	builder.WriteString("P\"")
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"':
			builder.WriteByte('\\')
			builder.WriteByte(s[i])
		case '\033':
			builder.WriteString("\\e")
		case '\n':
			builder.WriteString("\\n")
		case '\r':
			builder.WriteString("\\r")
		case '\t':
			builder.WriteString("\\t")
		default:
			if s[i] >= 32 && s[i] <= 126 {
				builder.WriteByte(s[i])
			} else {
				builder.WriteString(fmt.Sprintf("\\x%02x", s[i]))
			}
		}
	}
	// Close the format string.
	builder.WriteByte('"')
	return builder.String()
}

// RandomSSID returns a random SSID of length 30 and given prefix.
func RandomSSID(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Generate 30-char SSID, including prefix
	n := 30 - len(prefix)
	s := make([]byte, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return prefix + string(s)
}

const macBitLocal = 0x2
const macBitMulticast = 0x1

// RandomMAC returns a random MAC address for WiFi device.
// The MAC address is a locally administered unicast address.
// This can also be used as BSSID.
func RandomMAC() (net.HardwareAddr, error) {
	randMAC := make(net.HardwareAddr, 6)
	if _, err := rand.Read(randMAC); err != nil {
		return nil, errors.Wrap(err, "failed to generate random MAC address")
	}
	randMAC[0] = (randMAC[0] &^ macBitMulticast) | macBitLocal
	return randMAC, nil
}
