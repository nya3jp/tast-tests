// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
)

var htTable = map[string]string{
	"no secondary": "HT20",
	"above":        "HT40+",
	"below":        "HT40-",
}

const (
	securityOpen  = "open"
	securityWEP   = "wep"
	securityWPA   = "wpa"
	securityWPA2  = "wpa2"
	securityMixed = "mixed"
)

// IfType is the type of WiFi interface.
type IfType string

// IfType enums. (Only defines the values useful for us. For complete
// list of possible values, please refer to iftype_name in iw.)
// NOTE: When adding new types, please also update the busy type logic
// in remote/wificell/router.go
const (
	IfTypeManaged IfType = "managed"
	IfTypeMonitor IfType = "monitor"
)

// The iw link keys.
const (
	LinkKeyBeaconInterval = "beacon int"
	LinkKeyDtimPeriod     = "dtim period"
	LinkKeyFrequency      = "freq"
	LinkKeySignal         = "signal"
	LinkKeyRxBitrate      = "rx bitrate"
	LinkKeyRxDrops        = "rx drop misc"
	LinkKeyRxPackets      = "rx packets"
	LinkKeyTxBitrate      = "tx bitrate"
	LinkKeyTxFailures     = "tx failed"
	LinkKeyTxPackets      = "tx packets"
	LinkKeyTxRetries      = "tx retries"
)

// Band contains supported wireless band attributes.
type Band struct {
	Num            int
	FrequencyFlags map[int][]string
	MCSIndices     []int
}

// BSSData contains contents pertaining to a BSS response.
type BSSData struct {
	BSS       string
	Frequency int
	SSID      string
	Security  string
	HT        string
	Signal    float64
}

// NetDev contains interface attributes from `iw dev`.
type NetDev struct {
	PhyNum int
	IfName string
	IfType IfType
}

// String implements the Stringer interface for NetDev.
func (n NetDev) String() string {
	return fmt.Sprintf("{phy=%d, name=%s, type=%s}", n.PhyNum, n.IfName, n.IfType)
}

// Phy contains phy# attributes.
type Phy struct {
	Name                      string
	Bands                     []Band
	Modes, Commands, Features []string
	RxAntenna, TxAntenna      int
	MaxScanSSIDs              int
	SupportVHT                bool
	SupportHT2040             bool
	SupportHT20SGI            bool
	SupportHT40SGI            bool
	SupportVHT80SGI           bool
	SupportMUMIMO             bool
}

// SupportSetAntennaMask tells if we can set the antenna bitmap on the Phy.
func (p *Phy) SupportSetAntennaMask() bool {
	return p.RxAntenna != 0 && p.TxAntenna != 0
}

// ChannelConfig contains the configuration data for a radio config.
type ChannelConfig struct {
	Number, Freq, Width, Center1Freq int
}

type section struct {
	header, body string
}

// sectionAttributes contains temporary results while parsing sections.
// Sections are defined as blocks of text that are delimited by level 1 indent lines.
// e.g.
//	Band 1:
//		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
//		Minimum RX AMPDU time spacing: 4 usec (0x05)
// The 2nd and 3rd lines belong to the section of "Band 1".
type sectionAttributes struct {
	bands                 []Band
	phyModes, phyCommands []string
	supportVHT            bool
	supportHT2040         bool
	supportHT20SGI        bool
	supportHT40SGI        bool
	supportVHT80SGI       bool
	supportMUMIMO         bool
}

// TimedScanData contains the BSS responses from an `iw scan` and its execution time.
type TimedScanData struct {
	Time    time.Duration
	BSSList []*BSSData
}

// Runner is the object contains iw utilities.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates a new iw command utility runner.
func NewRunner(c cmd.Runner) *Runner {
	return &Runner{cmd: c}
}

// InterfaceAttributes gets the interface's attributes.
func (r *Runner) InterfaceAttributes(ctx context.Context, iface string) (*NetDev, error) {
	var matchIfs []*NetDev
	ifs, err := r.ListInterfaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, val := range ifs {
		if val.IfName == iface {
			matchIfs = append(matchIfs, val)
		}
	}
	if len(matchIfs) == 0 {
		return nil, errors.Errorf("could not find an interface named %s", iface)
	}
	if len(matchIfs) > 1 {
		return nil, errors.Errorf("multiple interfaces named %s", iface)
	}
	return matchIfs[0], nil
}

// ListInterfaces yields all the attributes (NetDev) for each interface.
func (r *Runner) ListInterfaces(ctx context.Context) ([]*NetDev, error) {
	out, err := r.cmd.Output(ctx, "iw", "dev")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list interfaces with command \"iw dev\"")
	}

	return parseInterfaces(string(out))
}

func parseInterfaces(iwOut string) ([]*NetDev, error) {
	var interfaces []*NetDev

	sections, err := parseSection(`(?m)^phy#([0-9]+)$`, iwOut)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse a NetDev from \"iw dev\" output")
	}
	for _, sec := range sections {
		phy := sec.header
		ifaces, err := parseSection(`(?m)^\s*Interface (.*)$`, sec.body)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse interface")
		}
		for _, iface := range ifaces {
			// iface.body has a leading newline, but newNetDev handles that gracefully.
			netdev, err := newNetDev(phy, iface.header, iface.body)
			if err != nil {
				return nil, errors.Wrap(err, "could not extract interface attributes")
			}
			interfaces = append(interfaces, netdev)
		}
	}
	return interfaces, nil
}

// ListPhys returns a list of Phy struct for each phy on the DUT.
// Note that it returns an empty list without error when "iw list" command returns nothing.
// Client must check []*Phy's length before accessing it.
func (r *Runner) ListPhys(ctx context.Context) ([]*Phy, error) {
	out, err := r.cmd.Output(ctx, "iw", "list")
	if err != nil {
		return nil, errors.Wrap(err, "iw list failed")
	}

	sections, err := parseSection(`Wiphy (.*)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse phys")
	}
	var phys []*Phy
	for _, sec := range sections {
		phy, err := newPhy(sec.header, sec.body)
		if err != nil {
			return nil, errors.Wrap(err, "could not extract phy attributes")
		}
		phys = append(phys, phy)
	}
	return phys, nil
}

// PhyByName returns a Phy struct for the given name.
func (r *Runner) PhyByName(ctx context.Context, name string) (*Phy, error) {
	out, err := r.cmd.Output(ctx, "iw", "phy", name, "info")
	if err != nil {
		return nil, errors.Wrapf(err, `"iw phy %s info" failed`, name)
	}
	return parsePhyInfo(out)
}

// PhyByID returns a Phy struct for the given phy id.
func (r *Runner) PhyByID(ctx context.Context, id int) (*Phy, error) {
	out, err := r.cmd.Output(ctx, "iw", fmt.Sprintf("phy#%d", id), "info")
	if err != nil {
		return nil, errors.Wrapf(err, "\"iw phy#%d info\" failed", id)
	}
	return parsePhyInfo(out)
}

// TimedScan runs a scan on a specified interface and frequencies (if applicable).
// A channel map for valid frequencies can be found in
// third_party/autotest/files/server/cros/network/hostap_config.py
// The frequency slice is used to allow which frequencies/bands to scan on.
// The SSIDs slice will filter the results of the scan to those that pertain
// to the allowed SSIDs (although this doesn't seem to work on some devices).
func (r *Runner) TimedScan(ctx context.Context, iface string,
	frequencies []int, ssids []string) (*TimedScanData, error) {
	args := []string{"dev", iface, "scan"}
	for _, freq := range frequencies {
		args = append(args, "freq", strconv.Itoa(freq))
	}
	for _, ssid := range ssids {
		args = append(args, "ssid", ssid)
	}
	startTime := time.Now()
	out, err := r.cmd.Output(ctx, "iw", args...)
	scanTime := time.Since(startTime)
	if err != nil {
		return nil, errors.Wrap(err, "iw scan failed")
	}
	scanOut := string(out)
	bssList, err := parseScanResults(scanOut)
	if err != nil {
		return nil, err
	}
	return &TimedScanData{scanTime, bssList}, nil
}

// ScanDump returns a list of BSSData from a scan dump.
func (r *Runner) ScanDump(ctx context.Context, iface string) ([]*BSSData, error) {
	out, err := r.cmd.Output(ctx, "iw", "dev", iface, "scan", "dump")
	if err != nil {
		return nil, errors.Wrap(err, "scan dump failed")
	}
	return parseScanResults(string(out))
}

// CurrentBSSID gets the BSS ID the interface associated with from iw link output.
func (r *Runner) CurrentBSSID(ctx context.Context, iface string) (string, error) {
	res, err := r.cmd.Output(ctx, "iw", "dev", iface, "link")
	if err != nil {
		return "", errors.Wrapf(err, "failed to get link information from interface %s", iface)
	}
	return extractBSSID(string(res))
}

// LinkValue gets the specified link value from the iw link output.
func (r *Runner) LinkValue(ctx context.Context, iface, iwLinkKey string) (string, error) {
	res, err := r.cmd.Output(ctx, "iw", "dev", iface, "link")
	if err != nil {
		return "", errors.Wrapf(err, "failed to get link information from interface %s", iface)
	}
	kvs := getAllLinkKeys(string(res))
	out := kvs[iwLinkKey]
	if out == "" {
		return "", errors.Errorf("could not extract link value from link information with link key %s: %v", iwLinkKey, kvs)
	}
	return out, nil
}

// OperatingMode gets the interface's operating mode.
func (r *Runner) OperatingMode(ctx context.Context, iface string) (string, error) {
	out, err := r.cmd.Output(ctx, "iw", "dev", iface, "info")
	if err != nil {
		return "", errors.Wrap(err, "failed to get interface information")
	}
	supportedDevModes := []string{"AP", "monitor", "managed"}
	m, err := extractMatch(`(?m)^\s*type (.*)$`, string(out))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse operating mode")
	}
	opMode := m[0]
	for _, v := range supportedDevModes {
		if v == opMode {
			return opMode, nil
		}
	}
	return "", errors.Wrapf(err, "unsupported operating mode %s found for interface: %s", opMode, iface)
}

// PowersaveMode returns true if powersave mode is enabled and false if it is disabled.
func (r *Runner) PowersaveMode(ctx context.Context, iface string) (bool, error) {
	out, err := r.cmd.Output(ctx, "iw", "dev", iface, "get", "power_save")
	if err != nil {
		return false, errors.Wrap(err, "failed to get powersave mode")
	}

	m, err := extractMatch(`Power save:\s+(\w+)`, string(out))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse powersave mode")
	}

	psMode := m[0]
	if psMode == "on" {
		return true, nil
	} else if psMode == "off" {
		return false, nil
	}

	return false, errors.Wrapf(err, "unexpected powersave mode: got %s; want [on, off]", psMode)
}

// SetPowersaveMode sets the powersave mode.
func (r *Runner) SetPowersaveMode(ctx context.Context, iface string, turnOn bool) error {
	mode := "off"
	if turnOn {
		mode = "on"
	}

	if err := r.cmd.Run(ctx, "iw", "dev", iface, "set", "power_save", mode); err != nil {
		return errors.Wrapf(err, "failed to set the powersave mode %s", mode)
	}

	return nil
}

// RadioConfig gets the radio configuration from the interface's information.
func (r *Runner) RadioConfig(ctx context.Context, iface string) (*ChannelConfig, error) {
	out, err := r.cmd.Output(ctx, "iw", "dev", iface, "info")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface information")
	}
	m, err := extractMatch(`(?m)^\s*channel (\d+) \((\d+) MHz\), width: (\d{2}) MHz, center1: (\d+) MHz`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pase radio config")
	}
	number, err := strconv.Atoi(m[0])
	if err != nil {
		return nil, errors.New("could not parse number")
	}
	freq, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.New("could not parse freq")
	}
	width, err := strconv.Atoi(m[2])
	if err != nil {
		return nil, errors.New("could not parse width")
	}
	center1Freq, err := strconv.Atoi(m[3])
	if err != nil {
		return nil, errors.New("could not parse center1Freq")
	}
	return &ChannelConfig{
		Number:      number,
		Freq:        freq,
		Width:       width,
		Center1Freq: center1Freq,
	}, nil
}

// RegulatoryDomain gets the regulatory domain code.
func (r *Runner) RegulatoryDomain(ctx context.Context) (string, error) {
	out, err := r.cmd.Output(ctx, "iw", "reg", "get")
	if err != nil {
		return "", errors.Wrap(err, "failed to get regulatory domain")
	}
	re := regexp.MustCompile(`(?m)^country (..):`)
	if m := re.FindStringSubmatch(string(out)); m != nil {
		return m[1], nil
	}
	return "", errors.New("could not find regulatory domain")
}

// IsRegulatorySelfManaged determines if any WiFi device on the system manages its own
// regulatory info (NL80211_ATTR_WIPHY_SELF_MANAGED_REG).
func (r *Runner) IsRegulatorySelfManaged(ctx context.Context) (bool, error) {
	out, err := r.cmd.Output(ctx, "iw", "reg", "get")
	if err != nil {
		return false, errors.Wrap(err, "failed to get regulatory domain")
	}
	re := regexp.MustCompile(`(?m)^phy#.*\(self-managed\)`)
	if m := re.FindStringSubmatch(string(out)); m != nil {
		return true, nil
	}
	return false, nil
}

// SetRegulatoryDomain sets the regulatory domain code.
// country is ISO/IEC 3166-1 alpha2 code for the country.
func (r *Runner) SetRegulatoryDomain(ctx context.Context, country string) error {
	if err := r.cmd.Run(ctx, "iw", "reg", "set", country); err != nil {
		return errors.Wrapf(err, "failed to set regulatory domain %s", country)
	}
	return nil
}

// SetTxPower sets the wireless interface's transmit power.
// mode: 'fixed' or 'limit'
// power: power in mBm (milli-Bel-milliwatts). 1 mBm = 100 * dBm (deci-Bell-milliwatts).
func (r *Runner) SetTxPower(ctx context.Context, iface, mode string, power int) error {
	if mode != "fixed" && mode != "limit" {
		return errors.Errorf("unexpected mode = %q", mode)
	}
	if err := r.cmd.Run(ctx, "iw", "dev", iface, "set", "txpower", mode, strconv.Itoa(power)); err != nil {
		return errors.Wrap(err, "failed to set txpower")
	}
	return nil
}

// SetTxPowerAuto sets the wireless interface's transmit power to auto mode.
func (r *Runner) SetTxPowerAuto(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "iw", "dev", iface, "set", "txpower", "auto"); err != nil {
		return errors.Wrap(err, "failed to set txpower")
	}
	return nil
}

// ChWidth is the type of channel width setting (e.g. HT40+, 80 ...).
type ChWidth string

// ChWidth enum values.
// TODO(crbug.com/1017533, crbug.com/1005443): we can use 80MHz instead of 80
// if the routers in testlab are upgraded to have new enough iw (>4.14).
const (
	ChWidthNOHT      ChWidth = "NOHT"
	ChWidthHT20      ChWidth = "HT20"
	ChWidthHT40Plus  ChWidth = "HT40+"
	ChWidthHT40Minus ChWidth = "HT40-"
	ChWidth80        ChWidth = "80"
	ChWidth80P80     ChWidth = "80+80"
	ChWidth160       ChWidth = "160"
)

// setFreqConf contains the optional information for iw "set freq" function.
type setFreqConf struct {
	ctrlFreq    int
	width       ChWidth
	centerFreq1 int
	centerFreq2 int
}

// newSetFreqConf creates a setFreqConf with given options.
func newSetFreqConf(ctrlFreq int, ops ...SetFreqOption) (*setFreqConf, error) {
	conf := &setFreqConf{
		ctrlFreq: ctrlFreq,
		width:    ChWidthNOHT, // Default NOHT.
	}
	for _, op := range ops {
		op(conf)
	}
	if err := conf.init(); err != nil {
		return nil, err
	}
	return conf, nil
}

// init derives center frequency of 80, 160 MHz channel if not given and ensures the
// options fits the input requirement of iw "set freq" function.
// Format:
//   set freq <freq> [NOHT|HT20|HT40+|HT40-|5MHz|10MHz|80MHz]
//   set freq <control freq> [5|10|20|40|80|80+80|160] [<center1_freq> [<center2_freq>]]
// We use the second pattern for 80, 80+80 and 160, and the first one for the rest as iw
// will derive the center frequency for us so we don't have to duplicate the logic.
// This function does not validate the given frequencies here and delegate it to iw.
// (iw fails with invalid argument error if the frequencies are not matched or invalid.)
func (c *setFreqConf) init() error {
	switch c.width {
	case ChWidthNOHT, ChWidthHT20, ChWidthHT40Plus, ChWidthHT40Minus:
		// The center frequency can already be determined with control frequency
		// and channel width. Let's return error if center frequency is still set.
		if c.centerFreq1 != 0 || c.centerFreq2 != 0 {
			return errors.Errorf("don't specify center frequencies for width=%s", c.width)
		}
	case ChWidth80P80:
		// This needs both center frequency set.
		if c.centerFreq1 == 0 || c.centerFreq2 == 0 {
			return errors.Errorf("need both center frequencies for width=%s", c.width)
		}
	case ChWidth80:
		// This needs center frequency 1.
		if c.centerFreq1 == 0 {
			cf, err := c.centerFreq80(c.ctrlFreq)
			if err != nil {
				return err
			}
			c.centerFreq1 = cf
		}
		if c.centerFreq2 != 0 {
			return errors.Errorf("don't specify center frequency 2 for width=%s", c.width)
		}
	case ChWidth160:
		// This needs center frequency 1.
		if c.centerFreq1 == 0 {
			cf, err := c.centerFreq160(c.ctrlFreq)
			if err != nil {
				return err
			}
			c.centerFreq1 = cf
		}
		if c.centerFreq2 != 0 {
			return errors.Errorf("don't specify center frequency 2 for width=%s", c.width)
		}
	default:
		return errors.Errorf("invalid channel width=%s", c.width)
	}
	return nil
}

// centerFreq80 derives the center frequency (in MHz) for the channel with
// 80MHz width and control frequency = ctrlFreq MHz.
func (c *setFreqConf) centerFreq80(ctrlFreq int) (int, error) {
	vht80 := []int{5180, 5260, 5500, 5580, 5660, 5745}
	for _, f := range vht80 {
		if ctrlFreq >= f && ctrlFreq < f+80 {
			return f + 30, nil
		}
	}
	return 0, errors.Errorf("invalid control frequency %d for 80MHz channel width", ctrlFreq)
}

// centerFreq160 derives the center frequency (in MHz) for the channel with
// 160MHz width and control frequency = ctrlFreq MHz.
func (c *setFreqConf) centerFreq160(ctrlFreq int) (int, error) {
	vht160 := []int{5180, 5500}
	for _, f := range vht160 {
		if ctrlFreq >= f && ctrlFreq < f+160 {
			return f + 70, nil
		}
	}
	return 0, errors.Errorf("invalid control frequency %d for 160MHz channel width", ctrlFreq)
}

// toArgs formats the config to the arguments for iw "set freq" function.
// The argument format can be found in the doc of init()
func (c *setFreqConf) toArgs() []string {
	args := []string{strconv.Itoa(c.ctrlFreq)}
	switch c.width {
	case ChWidthHT20, ChWidthHT40Plus, ChWidthHT40Minus:
		args = append(args, string(c.width))
	case ChWidth80, ChWidth160:
		args = append(args, string(c.width), strconv.Itoa(c.centerFreq1))
	case ChWidth80P80:
		args = append(args, string(c.width), strconv.Itoa(c.centerFreq1), strconv.Itoa(c.centerFreq2))
	}
	// NOHT case, no extra argument needed.
	return args
}

// SetFreqOption is a function signature that modifies setFreqConf.
type SetFreqOption func(*setFreqConf)

// Equal checks if the effects of the two SetFreqOptions on an empty setFreqConfig
// are the same. This is useful for external package to write unit tests.
func (op SetFreqOption) Equal(other SetFreqOption) bool {
	c1 := &setFreqConf{}
	c2 := &setFreqConf{}
	op(c1)
	other(c2)
	return reflect.DeepEqual(c1, c2)
}

// String formats the option to string by the result of applying it on empty config.
// This is useful for external packages to write unit tests.
func (op SetFreqOption) String() string {
	c1 := &setFreqConf{}
	op(c1)
	return fmt.Sprintf("%v", c1)
}

// SetFreqChWidth returns a SetFreqOption which sets channel width.
func SetFreqChWidth(cw ChWidth) SetFreqOption {
	return func(c *setFreqConf) {
		c.width = cw
	}
}

// SetFreqCenterFreq1 returns a SetFreqOption which sets the first center frequency (in MHz).
func SetFreqCenterFreq1(f int) SetFreqOption {
	return func(c *setFreqConf) {
		c.centerFreq1 = f
	}
}

// SetFreqCenterFreq2 returns a SetFreqOption which sets the second center frequency (in MHz).
func SetFreqCenterFreq2(f int) SetFreqOption {
	return func(c *setFreqConf) {
		c.centerFreq2 = f
	}
}

// SetFreq sets the wireless interface's LO center freq.
// Interface should be in monitor mode before scanning.
func (r *Runner) SetFreq(ctx context.Context, iface string, freq int, ops ...SetFreqOption) error {
	conf, err := newSetFreqConf(freq, ops...)
	if err != nil {
		return err
	}
	args := []string{"dev", iface, "set", "freq"}
	args = append(args, conf.toArgs()...)
	if err := r.cmd.Run(ctx, "iw", args...); err != nil {
		return errors.Wrap(err, "failed to set freq")
	}
	return nil
}

// SetAntennaBitmap sets the antenna bitmap.
func (r *Runner) SetAntennaBitmap(ctx context.Context, phy string, txBitmap, rxBitmap int) error {
	if err := r.cmd.Run(ctx, "iw", "phy", phy, "set", "antenna", strconv.Itoa(txBitmap),
		strconv.Itoa(rxBitmap)); err != nil {
		return errors.Wrap(err, "failed to set Antenna bitmap")
	}
	return nil
}

// AddInterface creates a interface on phy with name=iface and type=t.
func (r *Runner) AddInterface(ctx context.Context, phy, iface string, t IfType) error {
	if err := r.cmd.Run(ctx, "iw", "phy", phy, "interface", "add", iface, "type", string(t)); err != nil {
		return errors.Wrapf(err, "failed to add interface %s on %s", iface, phy)
	}
	return nil
}

// RemoveInterface removes the iface.
func (r *Runner) RemoveInterface(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "iw", "dev", iface, "del"); err != nil {
		return errors.Wrapf(err, "failed to remove interface %s", iface)
	}
	return nil
}

// determineSecurity determines the security level of a connection based on the
// number of supported securities.
func determineSecurity(secs []string) string {
	if len(secs) == 0 {
		return securityOpen
	} else if len(secs) == 1 {
		return secs[0]
	} else {
		return securityMixed
	}
}

// extractBSSID parses the BSSID the interface associated with from the output
// of `iw dev $iface link`.
func extractBSSID(out string) (string, error) {
	r := regexp.MustCompile(`(?m)^Connected to ([0-9a-fA-F:]{17})`)
	m := r.FindStringSubmatch(out)
	if len(m) < 2 {
		return "", errors.New("no bssid found")
	}
	return m[1], nil
}

// getAllLinkKeys parses `link` or `station dump` output into key value pairs.
func getAllLinkKeys(out string) map[string]string {
	kv := make(map[string]string)
	r := regexp.MustCompile(`^\s+(.*):\s+(.*)$`)
	for _, line := range strings.Split(out, "\n") {
		m := r.FindStringSubmatch(line)
		if m != nil {
			kv[m[1]] = m[2]
		}
	}
	return kv
}

// newBSSData is a factory method which constructs a BSSData from individual
// scan entries.
// bssMatch is the BSSID line from the scan.
// dataMatch is the corresponding metadata associated with the BSS entry.
func newBSSData(bssMatch, dataMatch string) (*BSSData, error) {
	// Handle BSS.
	bssFields := strings.Fields(bssMatch)
	if len(bssFields) != 2 {
		return nil, errors.New("unexpected pattern for BSS match")
	}
	bss := bssFields[1]

	// Handle Frequency.
	m := regexp.MustCompile(`freq: (\d+)`).FindStringSubmatch(dataMatch)
	if m == nil {
		return nil, errors.New("freq field not found")
	}
	freq, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse freq field")
	}

	// Handle Signal Strength.
	sigMatch := regexp.MustCompile(`signal:.*`).FindString(dataMatch)
	sig, err := strconv.ParseFloat(strings.Fields(sigMatch)[1], 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse signal strength field")
	}

	// Handle SSID.
	ssidMatch := regexp.MustCompile(`SSID: (.+)`).FindStringSubmatch(dataMatch)
	ssid := ""
	if ssidMatch != nil {
		// No match = hidden SSID.
		ssid = ssidMatch[1]
	}

	// Handle high throughput setting.
	htMatch := regexp.MustCompile(
		`\* secondary channel offset.*`).FindString(dataMatch)
	htSplits := strings.Split(htMatch, ":")
	var ht string
	if len(htSplits) == 2 {
		htTemp, ok := htTable[strings.TrimSpace(htSplits[1])]
		if !ok {
			return nil, errors.Errorf("invalid HT entry parsed %s",
				strings.TrimSpace(htSplits[1]))
		}
		ht = htTemp
	} else {
		// Default high throughput value if the section is not advertised.
		ht = htTable["no secondary"]
	}

	// Handle Security.
	var secs []string
	if strings.Contains(dataMatch, "WPA") {
		secs = append(secs, "WPA")
	}
	if strings.Contains(dataMatch, "RSN") {
		secs = append(secs, "RSN")
	}
	sec := determineSecurity(secs)
	return &BSSData{
		BSS:       bss,
		Frequency: freq,
		SSID:      ssid,
		Security:  sec,
		HT:        ht,
		Signal:    sig}, nil
}

// newNetDev constructs a NetDev object from "iw dev" output.
func newNetDev(phyHeader, ifHeader, ifBody string) (*NetDev, error) {
	// Parse phy number.
	m, err := extractMatch(`^phy#([0-9]+)$`, phyHeader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse phy number")
	}
	phy, err := strconv.Atoi(m[0])
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert str %q to int", m[0])
	}

	// Parse interface name.
	m, err = extractMatch(`^\s*Interface (.*)$`, ifHeader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ifname")
	}
	ifName := m[0]

	// Parse ifType.
	m, err = extractMatch(`(?m)^\s*type ([a-zA-Z]+)$`, ifBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ifType")
	}
	ifType := IfType(m[0])

	return &NetDev{PhyNum: phy, IfName: ifName, IfType: ifType}, nil
}

func parsePhyTxRx(contents string) (tx, rx int, err error) {
	hexToInt := func(str string) (int, error) {
		res, err := strconv.ParseInt(str, 0, 64)
		if err != nil {
			return 0, errors.Wrap(err, "could not parse hex string")
		}
		return int(res), nil
	}
	var m []string
	m, err = extractMatch(`\s*Available Antennas: TX (\S+) RX (\S+)`, contents)
	if err != nil {
		err = errors.Wrap(err, "unable to find \"Available Antennas\"")
		return
	}

	tx, err = hexToInt(m[0])
	if err != nil {
		return
	}
	rx, err = hexToInt(m[1])
	if err != nil {
		tx = 0 // clear return value on error
		return
	}
	return
}

func parseDeviceSupport(contents string) ([]string, error) {
	var features []string
	matches := regexp.MustCompile(`\s*Device supports (.*)\.`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		features = append(features, m[1])
	}
	return features, nil
}

func parseMaxScanSSIDs(contents string) (int, error) {
	m, err := extractMatch(`\s*max # scan SSIDs: (\d+)`, contents)
	if err != nil {
		return 0, errors.Wrap(err, "unable to find \"max # scan SSIDs\"")
	}
	maxScanSSIDs, err := strconv.Atoi(m[0])
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert value of \"max # scan SSIDs\" to int: %s", m[0])
	}
	return maxScanSSIDs, nil
}

// newPhy constructs a Phy object from "iw list" output.
func newPhy(phyMatch, dataMatch string) (*Phy, error) {
	// Phy name handling.
	m, err := extractMatch(`Wiphy (.*)`, phyMatch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse name")
	}
	name := m[0]

	// Antennae handling.
	txAntenna, rxAntenna, err := parsePhyTxRx(dataMatch)
	if err != nil {
		return nil, err
	}

	// Device Support handling.
	phyFeatures, err := parseDeviceSupport(dataMatch)
	if err != nil {
		return nil, err
	}

	// Max Scan SSIDs handling.
	maxScanSSIDs, err := parseMaxScanSSIDs(dataMatch)
	if err != nil {
		return nil, err
	}

	// Handle parsing attributes that need to be handled on a section by section level.
	// Sections are defined as blocks of text that are delimited by level 1 indent lines.
	attrs, err := parseSectionSpecificAttributes(dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse all sections in parseSectionSpecificAttributes")
	}

	return &Phy{
		Name:            name,
		Bands:           attrs.bands,
		Modes:           attrs.phyModes,
		Commands:        attrs.phyCommands,
		Features:        phyFeatures,
		RxAntenna:       rxAntenna,
		TxAntenna:       txAntenna,
		MaxScanSSIDs:    maxScanSSIDs,
		SupportVHT:      attrs.supportVHT,
		SupportHT2040:   attrs.supportHT2040,
		SupportHT20SGI:  attrs.supportHT20SGI,
		SupportHT40SGI:  attrs.supportHT40SGI,
		SupportVHT80SGI: attrs.supportVHT80SGI,
		SupportMUMIMO:   attrs.supportMUMIMO,
	}, nil
}

// parsePhyInfo parses the output of "iw phy $phy info" or "iw phy#$id info".
func parsePhyInfo(out []byte) (*Phy, error) {
	// This has the same format as `iw list`, except that only one phy is printed.
	sections, err := parseSection(`Wiphy (.*)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse phys")
	}
	if len(sections) != 1 {
		return nil, errors.Errorf("got %d phy info sections, want 1", len(sections))
	}
	sec := sections[0]
	phy, err := newPhy(sec.header, sec.body)
	if err != nil {
		return nil, errors.Wrap(err, "could not extract phy attributes")
	}
	return phy, nil
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of BSSData structs.
func parseScanResults(output string) ([]*BSSData, error) {
	sections, err := parseSection(`BSS ([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse scan results")
	}
	var bssList []*BSSData
	for _, sec := range sections {
		data, err := newBSSData(sec.header, sec.body)
		if err != nil {
			return nil, err
		}
		bssList = append(bssList, data)
	}
	return bssList, nil
}

func parseBandMCSIndices(contents string) ([]int, error) {
	var ret []int
	r := regexp.MustCompile(`HT TX/RX MCS rate indexes supported: .*\n`)
	matches := r.FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		rateStr := strings.TrimSpace(strings.Split(m[0], ":")[1])
		for _, piece := range strings.Split(rateStr, ",") {
			piece = strings.TrimSpace(piece)
			if strings.Contains(piece, "-") {
				res := strings.SplitN(piece, "-", 2)
				begin, err := strconv.Atoi(res[0])
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse rate begin %q as int", res[0])
				}
				end, err := strconv.Atoi(res[1])
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse rate end %q as int", res[1])
				}
				for i := begin; i < end+1; i++ {
					ret = append(ret, i)
				}

			} else {
				val, err := strconv.Atoi(piece)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse rate %q as int", piece)
				}
				ret = append(ret, val)
			}
		}
	}
	return ret, nil
}

func parseFrequencyFlags(contents string) (map[int][]string, error) {
	ret := make(map[int][]string)
	r := regexp.MustCompile(`(?P<frequency>\d+) MHz \[\d+\](?: \([0-9.]+ dBm\))?(?: \((?P<flags>[a-zA-Z, ]+)\))?`)
	matches := r.FindAllStringSubmatch(contents, -1)
	var frequency int
	var err error
	for _, m := range matches {
		for i, tag := range r.SubexpNames() {
			if tag == "frequency" {
				frequency, err = strconv.Atoi(m[i])
				if err != nil {
					return nil, errors.Wrapf(err, "could not parse frequency %q as int", m[i])
				}
			} else if string(tag) == "flags" {
				flags := strings.Split(string(m[i]), ",")
				for i := range flags {
					flags[i] = strings.TrimSpace(flags[i])
				}
				if len(flags) > 0 && flags[0] != "" {
					ret[frequency] = flags
				} else {
					ret[frequency] = nil
				}
			}
		}
	}
	return ret, nil
}

func parseBand(attrs *sectionAttributes, sectionName, contents string) error {
	// This parser constructs a Band for the phy.
	var band Band

	// Band idx handling.
	m, err := extractMatch(`^Band (\d+):$`, sectionName)
	if err != nil {
		return errors.Wrap(err, "failed to parse band")
	}
	band.Num, err = strconv.Atoi(m[0])
	if err != nil {
		return errors.Wrapf(err, "could not parse band %q as int", m[0])
	}

	// Band rate handling.
	band.MCSIndices, err = parseBandMCSIndices(contents)
	if err != nil {
		return errors.Wrap(err, "failed to parse band rates")
	}

	// Band channel info handling.
	band.FrequencyFlags, err = parseFrequencyFlags(contents)
	if err != nil {
		return errors.Wrap(err, "failed to parse freqency flags")
	}

	attrs.bands = append(attrs.bands, band)
	return nil
}

func parseThroughput(attrs *sectionAttributes, sectionName, contents string) error {
	// This parser evaluates the throughput capabilities of the phy.
	// HT related.
	if strings.Contains(contents, "HT20/HT40") {
		attrs.supportHT2040 = true
	}
	if strings.Contains(contents, "RX HT20 SGI") {
		attrs.supportHT20SGI = true
	}
	if strings.Contains(contents, "RX HT40 SGI") {
		attrs.supportHT40SGI = true
	}
	// VHT related.
	if strings.Contains(contents, "VHT Capabilities") {
		attrs.supportVHT = true
	}
	if strings.Contains(contents, "short GI (80 MHz)") {
		attrs.supportVHT80SGI = true
	}
	if strings.Contains(contents, "MU Beamformee") {
		attrs.supportMUMIMO = true
	}
	return nil
}

func parseIfaceModes(attrs *sectionAttributes, sectionName, contents string) error {
	// This parser checks the supported interface modes for the phy.
	matches := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		attrs.phyModes = append(attrs.phyModes, m[1])
	}
	return nil
}

func parsePhyCommands(attrs *sectionAttributes, sectionName, contents string) error {
	// This parser checks the Phy's supported commands.
	matches := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		attrs.phyCommands = append(attrs.phyCommands, m[1])
	}
	return nil
}

var parsers = []struct {
	prefix string
	parse  func(attrs *sectionAttributes, sectionName, contents string) error
}{
	{
		prefix: "Band",
		parse:  parseBand,
	},
	{
		prefix: "Band",
		parse:  parseThroughput,
	},
	{
		prefix: "Supported interface modes",
		parse:  parseIfaceModes,
	},
	{
		prefix: "Supported commands",
		parse:  parsePhyCommands,
	},
}

func parseSectionSpecificAttributes(output string) (*sectionAttributes, error) {
	attrs := sectionAttributes{}
	sections, err := parseSection(`(?m)^\t(\w.*):\s*$`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse sections")
	}
	// For each section, try to parse it with available parsers and stores
	// the parsed result to sectionAttribute.
	for _, sec := range sections {
		m := strings.TrimSpace(sec.header)
		for _, parser := range parsers {
			if !strings.HasPrefix(m, parser.prefix) {
				continue
			}
			if err := parser.parse(&attrs, m, sec.body); err != nil {
				return nil, err
			}
		}
	}
	return &attrs, nil
}

func extractMatch(regex, text string) ([]string, error) {
	r := regexp.MustCompile(regex)
	m := r.FindStringSubmatch(text)
	if len(m) != r.NumSubexp()+1 {
		return nil, errors.New("could not parse MatchGroup")
	}
	return m[1:], nil
}

// parseSection splits the text into sections by the specified regex as
// header. The first split without a header is discarded so that section
// headers and bodies are paired.
func parseSection(regex, text string) ([]section, error) {
	r := regexp.MustCompile(regex)
	matches := r.FindAllString(text, -1)
	bodies := r.Split(text, -1)
	if len(bodies) != len(matches)+1 {
		return nil, errors.New("unexpected number of matches")
	}
	bodies = bodies[1:]

	sections := make([]section, len(matches))
	for i := range sections {
		sections[i] = section{
			header: matches[i],
			body:   bodies[i],
		}
	}

	return sections, nil
}
