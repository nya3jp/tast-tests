// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// stubCmdRunner is a simple stub of CmdRunner which always returns the given content
// as command output. This is useful for testing some simple parsing that is not
// extracted as an independent function.
type stubCmdRunner struct {
	out []byte
}

// Run is a noop mock which always returns nil.
func (r *stubCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return nil
}

// Output is a mock which pretends the command is executed successfully and prints
// the pre-assigned output.
func (r *stubCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return r.out, nil
}

// CreateCmd is a mock function which does nothing.
func (r *stubCmdRunner) CreateCmd(ctx context.Context, cmd string, args ...string) {
	return
}

// SetStdOut is a mock function which does nothing.
func (r *stubCmdRunner) SetStdOut(stdoutFile *os.File) {
	return
}

// StderrPipe is a mock function which always returns nil.
func (r *stubCmdRunner) StderrPipe() (io.ReadCloser, error) {
	return nil, nil
}

// StartCmd is a mock function which always returns nil.
func (r *stubCmdRunner) StartCmd() error {
	return nil
}

// WaitCmd is a mock function which always returns nil.
func (r *stubCmdRunner) WaitCmd() error {
	return nil
}

// CmdExists is a mock function which always returns false.
func (r *stubCmdRunner) CmdExists() bool {
	return false
}

// ReleaseProcess is a mock function which always returns nil.
func (r *stubCmdRunner) ReleaseProcess() error {
	return nil
}

// ResetCmd is a mock function which does nothing.
func (r *stubCmdRunner) ResetCmd() {
	return
}

func TestAllLinkKeys(t *testing.T) {
	const testStr = `Connected to 74:e5:43:10:4f:c0 (on wlan0)
      SSID: PMKSACaching_4m9p5_ch1
      freq: 5220
      RX: 5370 bytes (37 packets)
      TX: 3604 bytes (15 packets)
      signal: -59 dBm
      tx bitrate: 13.0 MBit/s MCS 1

      bss flags:      short-slot-time
      dtim period:    5
      beacon int:     100`
	cmpMap := map[string]string{
		"SSID":        "PMKSACaching_4m9p5_ch1",
		"freq":        "5220",
		"TX":          "3604 bytes (15 packets)",
		"signal":      "-59 dBm",
		"bss flags":   "short-slot-time",
		"dtim period": "5",
		"beacon int":  "100",
		"RX":          "5370 bytes (37 packets)",
		"tx bitrate":  "13.0 MBit/s MCS 1",
	}
	l := allLinkKeys(testStr)

	if !reflect.DeepEqual(l, cmpMap) {
		t.Errorf("unexpected result in allLinkKeys: got %v, want %v", l, cmpMap)
	}
}

func TestParseScanResults(t *testing.T) {
	const testStr = `BSS 00:11:22:33:44:55(on wlan0)
	freq: 2447
	beacon interval: 100 TUs
	signal: -46.00 dBm
	Information elements from Probe Response frame:
	SSID: my_wpa2_network
	Extended supported rates: 24.0 36.0 48.0 54.0
	HT capabilities:
		Capabilities: 0x0c
			HT20
	HT operation:
		 * primary channel: 8
		 * secondary channel offset: no secondary
		 * STA channel width: 20 MHz
	RSN:	 * Version: 1
		 * Group cipher: CCMP
		 * Pairwise ciphers: CCMP
		 * Authentication suites: PSK
		 * Capabilities: 1-PTKSA-RC 1-GTKSA-RC (0x0000)
`
	l, err := parseScanResults(testStr)
	if err != nil {
		t.Fatal("parseScanResults failed: ", err)
	}
	cmpBSS := []*BSSData{
		{
			BSS:       "00:11:22:33:44:55",
			Frequency: 2447,
			SSID:      "my_wpa2_network",
			Security:  "RSN",
			HT:        "HT20",
			Signal:    -46,
		},
	}
	if !reflect.DeepEqual(l, cmpBSS) {
		t.Errorf("unexpected result in parseScanResults: got %v, want %v", l, cmpBSS)
	}
}

func TestNewPhy(t *testing.T) {
	testcases := []struct {
		header  string
		section string
		expect  *Phy
	}{
		{
			header: `Wiphy 3`,
			section: `	max # scan SSIDs: 20
	max scan IEs length: 425 bytes
	max # sched scan SSIDs: 20
	max # match sets: 11
	Retry short limit: 7
	Retry long limit: 4
	Coverage class: 0 (up to 0m)
	Device supports RSN-IBSS.
	Device supports AP-side u-APSD.
	Device supports T-DLS.
	Supported Ciphers:
		* WEP40 (00-0f-ac:1)
		* WEP104 (00-0f-ac:5)
		* TKIP (00-0f-ac:2)
		* CCMP-128 (00-0f-ac:4)
		* CMAC (00-0f-ac:6)
	Available Antennas: TX 0 RX 0
	Supported interface modes:
		 * IBSS
		 * managed
		 * monitor
	Band 1:
		Capabilities: 0x11ef
			RX LDPC
			HT20/HT40
			SM Power Save disabled
			RX HT20 SGI
			RX HT40 SGI
			TX STBC
			RX STBC 1-stream
			Max AMSDU length: 3839 bytes
			DSSS/CCK HT40
		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
		Minimum RX AMPDU time spacing: 4 usec (0x05)
		HT Max RX data rate: 300 Mbps
		HT TX/RX MCS rate indexes supported: 0-15
		Bitrates (non-HT):
			* 1.0 Mbps
		Frequencies:
			* 2412 MHz [1] (22.0 dBm)
	Supported commands:
		 * connect
		 * disconnect
	valid interface combinations:
		 * #{ managed } <= 2, #{ AP, P2P-client, P2P-GO } <= 2, #{ P2P-device } <= 1,
		   total <= 4, #channels <= 1
		 * #{ managed } <= 2, #{ P2P-client } <= 2, #{ AP, P2P-GO } <= 1, #{ P2P-device } <= 1,
		   total <= 4, #channels <= 2
		 * #{ managed } <= 1, #{ outside context of a BSS, mesh point, IBSS } <= 1,
		   total <= 2, #channels <= 1`,
			expect: &Phy{
				Name: "3",
				Bands: []Band{
					{
						Num: 1,
						FrequencyFlags: map[int][]string{
							2412: nil,
						},
						MCSIndices: []int{
							0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
						},
					},
				},
				Modes: []string{
					"IBSS",
					"managed",
					"monitor",
				},
				Commands: []string{
					"connect",
					"disconnect",
				},
				Features: []string{
					"RSN-IBSS",
					"AP-side u-APSD",
					"T-DLS",
				},
				RxAntenna:       0,
				TxAntenna:       0,
				MaxScanSSIDs:    20,
				SupportVHT:      false,
				SupportHT2040:   true,
				SupportHT20SGI:  true,
				SupportHT40SGI:  true,
				SupportVHT80SGI: false,
				SupportMUMIMO:   false,
				IfaceCombinations: []IfaceCombination{
					{
						IfaceLimits: []IfaceLimit{
							{
								IfaceTypes: []IfType{
									IfTypeManaged,
								},
								MaxCount: 2,
							},
							{
								IfaceTypes: []IfType{
									IfTypeAP,
									IfTypeP2PClient,
									IfTypeP2PGO,
								},
								MaxCount: 2,
							},
							{
								IfaceTypes: []IfType{
									IfTypeP2PDevice,
								},
								MaxCount: 1,
							},
						},
						MaxTotal:    4,
						MaxChannels: 1,
					},
					{
						IfaceLimits: []IfaceLimit{
							{
								IfaceTypes: []IfType{
									IfTypeManaged,
								},
								MaxCount: 2,
							},
							{
								IfaceTypes: []IfType{
									IfTypeP2PClient,
								},
								MaxCount: 2,
							},
							{
								IfaceTypes: []IfType{
									IfTypeAP,
									IfTypeP2PGO,
								},
								MaxCount: 1,
							},
							{
								IfaceTypes: []IfType{
									IfTypeP2PDevice,
								},
								MaxCount: 1,
							},
						},
						MaxTotal:    4,
						MaxChannels: 2,
					},
					{
						IfaceLimits: []IfaceLimit{
							{
								IfaceTypes: []IfType{
									IfTypeManaged,
								},
								MaxCount: 1,
							},
							{
								IfaceTypes: []IfType{
									IfTypeOutsideContextOfBSS,
									IfTypeMeshPoint,
									IfTypeIBSS,
								},
								MaxCount: 1,
							},
						},
						MaxTotal:    2,
						MaxChannels: 1,
					},
				},
			},
		},
		{
			header: `Wiphy phy0`,
			section: `	wiphy index: 0
	max # scan SSIDs: 16
	max scan IEs length: 195 bytes
	max # sched scan SSIDs: 0
	max # match sets: 0
	max # scan plans: 1
	max scan plan interval: -1
	max scan plan iterations: 0
	Retry short limit: 7
	Retry long limit: 4
	Coverage class: 0 (up to 0m)
	Device supports RSN-IBSS.
	Device supports AP-side u-APSD.
	Supported Ciphers:
		* WEP40 (00-0f-ac:1)
		* WEP104 (00-0f-ac:5)
		* TKIP (00-0f-ac:2)
		* CCMP-128 (00-0f-ac:4)
		* CMAC (00-0f-ac:6)
		* CMAC-256 (00-0f-ac:13)
		* GMAC-128 (00-0f-ac:11)
		* GMAC-256 (00-0f-ac:12)
	Available Antennas: TX 0x3 RX 0x3
	Configured Antennas: TX 0x3 RX 0x3
	Supported interface modes:
		 * managed
		 * AP
		 * monitor
	Band 2:
		Capabilities: 0x19ef
			RX LDPC
			HT20/HT40
			SM Power Save disabled
			RX HT20 SGI
			RX HT40 SGI
			TX STBC
			RX STBC 1-stream
			Max AMSDU length: 7935 bytes
			DSSS/CCK HT40
		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
		Minimum RX AMPDU time spacing: 8 usec (0x06)
		HT TX/RX MCS rate indexes supported: 0-15
		VHT Capabilities (0x339071b2):
			Max MPDU length: 11454
			Supported Channel Width: neither 160 nor 80+80
			RX LDPC
			short GI (80 MHz)
			TX STBC
			SU Beamformee
			MU Beamformee
			RX antenna pattern consistency
			TX antenna pattern consistency
		VHT RX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: not supported
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT RX highest supported: 0 Mbps
		VHT TX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: not supported
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT TX highest supported: 0 Mbps
		Bitrates (non-HT):
			* 6.0 Mbps
			* 9.0 Mbps
			* 12.0 Mbps
			* 18.0 Mbps
			* 24.0 Mbps
			* 36.0 Mbps
			* 48.0 Mbps
			* 54.0 Mbps
		Frequencies:
			* 5180 MHz [36] (23.0 dBm)
	Supported commands:
		 * new_interface
		 * set_interface
			`,
			expect: &Phy{
				Name: "phy0",
				Bands: []Band{
					{
						Num: 2,
						FrequencyFlags: map[int][]string{
							5180: nil,
						},
						MCSIndices: []int{
							0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
						},
					},
				},
				Modes: []string{
					"managed",
					"AP",
					"monitor",
				},
				Commands: []string{
					"new_interface",
					"set_interface",
				},
				Features: []string{
					"RSN-IBSS",
					"AP-side u-APSD",
				},
				RxAntenna:         3,
				TxAntenna:         3,
				MaxScanSSIDs:      16,
				SupportVHT:        true,
				SupportHT2040:     true,
				SupportHT20SGI:    true,
				SupportHT40SGI:    true,
				SupportVHT80SGI:   true,
				SupportMUMIMO:     true,
				IfaceCombinations: []IfaceCombination(nil),
			},
		},
	}
	for i, tc := range testcases {
		l, err := newPhy(tc.header, tc.section)
		if err != nil {
			t.Errorf("testcase #%d: newPhy failed: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(l, tc.expect) {
			t.Errorf("testcase #%d: unexpected result in newPhy: got %v, want %v", i, l, tc.expect)
		}
	}
}

func TestParseHiddenScanResults(t *testing.T) {
	const testStr = `BSS 00:11:22:33:44:55(on wlan0)
	freq: 2412
	beacon interval: 100 TUs
	signal: -46.00 dBm
	Information elements from Probe Response frame:
	SSID: 
	Supported rates: 1.0* 2.0* 5.5* 11.0* 6.0 9.0 12.0 18.0
	Extended supported rates: 24.0 36.0 48.0 54.0
	HT capabilities:
		Capabilities: 0x0c
			HT20
	HT operation:
		 * primary channel: 8
		 * secondary channel offset: no secondary
		 * STA channel width: 20 MHz
`
	l, err := parseScanResults(testStr)
	if err != nil {
		t.Fatal("parseScanResults failed: ", err)
	}
	cmpBSS := []*BSSData{
		{
			BSS:       "00:11:22:33:44:55",
			Frequency: 2412,
			SSID:      "",
			Security:  "open",
			HT:        "HT20",
			Signal:    -46,
		},
	}
	if diff := cmp.Diff(l, cmpBSS); diff != "" {
		t.Error("parseScanResults returned unexpected result; diff:\n", diff)
	}
}

func TestParseBandMCSIndices(t *testing.T) {
	// Partial data from elm DUT.
	content := `
                Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
                Minimum RX AMPDU time spacing: No restriction (0x00)
                HT TX/RX MCS rate indexes supported: 0-15, 32
	`
	expected := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 32}
	ret, err := parseBandMCSIndices(content)
	if err != nil {
		t.Fatal("parseBandMCSIndices failed: ", err)
	}
	if !reflect.DeepEqual(ret, expected) {
		t.Errorf("unexpected result in parseBandMCSIndices: got %v, want %v", ret, expected)
	}
}

func TestParseFrequencyFlags(t *testing.T) {
	// Hand-crafted data to test different cases.
	content := `
                Frequencies:
                        * 5040 MHz [8] (disabled)
                        * 5190 MHz [38] (23.0 dBm)
                        * 5210 MHz [42] (23.0 dBm) (passive scan, radar detection)
	`
	expected := map[int][]string{
		5040: {"disabled"},
		5190: nil,
		5210: {"passive scan", "radar detection"},
	}
	ret, err := parseFrequencyFlags(content)
	if err != nil {
		t.Fatal("parseFrequencyFlags failed: ", err)
	}
	if !reflect.DeepEqual(ret, expected) {
		t.Errorf("unexpected result in parseFrequencyFlags: got %v, want %v", ret, expected)
	}
}

func TestParseInterfaces(t *testing.T) {
	for _, param := range []struct {
		content  string
		expected []*NetDev
	}{
		{
			content: `phy#1
	Interface managed0
		ifindex 142
		wdev 0x100000080
		addr 00:11:22:33:44:55
		type managed
	Interface monitor0
		ifindex 141
		wdev 0x10000007f
		addr 00:11:22:33:44:55
		type monitor
phy#0
	Interface managed2
		ifindex 139
		wdev 0x9
		addr 00:11:22:33:44:55
		type managed
`,
			expected: []*NetDev{
				{
					PhyNum: 1,
					IfName: "managed0",
					IfType: "managed",
				},
				{
					PhyNum: 1,
					IfName: "monitor0",
					IfType: "monitor",
				},
				{
					PhyNum: 0,
					IfName: "managed2",
					IfType: "managed",
				},
			},
		},
		{
			content: `phy#0
	Interface wlan0
		ifindex 8
		wdev 0x100000001
		addr 50:00:00:00:00:01
		type managed
		channel 52 (5260 MHz), width: 40 MHz, center1: 5270 MHz
		txpower 23.00 dBm
`,
			expected: []*NetDev{
				{
					PhyNum: 0,
					IfName: "wlan0",
					IfType: "managed",
				},
			},
		},
	} {
		devs, err := parseInterfaces(param.content)
		if err != nil {
			t.Fatal("parseInterfaces failed: ", err)
		}
		if !reflect.DeepEqual(devs, param.expected) {
			t.Errorf("unexpected result in parseInterfaces: got %v, want %v", devs, param.expected)
		}
	}
}

func TestSetFreqOption(t *testing.T) {
	testcases := []struct {
		ctrlFreq int
		ops      []SetFreqOption
		valid    bool
		args     []string
	}{
		{
			ctrlFreq: 2412,
			ops:      nil,
			valid:    true,
			args:     []string{"2412"},
		},
		{
			ctrlFreq: 2412,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidthHT20)},
			valid:    true,
			args:     []string{"2412", "HT20"},
		},
		{
			ctrlFreq: 2412,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidthHT40Plus)},
			valid:    true,
			args:     []string{"2412", "HT40+"},
		},
		{
			ctrlFreq: 5240,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidthHT40Minus)},
			valid:    true,
			args:     []string{"5240", "HT40-"},
		},
		{
			ctrlFreq: 5180,
			ops: []SetFreqOption{
				SetFreqChWidth(ChWidthHT40Plus),
				SetFreqCenterFreq1(5190),
			},
			valid: false,
		},
		{
			ctrlFreq: 5240,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidth80)},
			valid:    true,
			args:     []string{"5240", "80", "5210"},
		},
		{
			ctrlFreq: 5240,
			ops: []SetFreqOption{
				SetFreqChWidth(ChWidth80),
				SetFreqCenterFreq1(5210),
			},
			valid: true,
			args:  []string{"5240", "80", "5210"},
		},
		{
			ctrlFreq: 5240,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidth160)},
			valid:    true,
			args:     []string{"5240", "160", "5250"},
		},
		{
			ctrlFreq: 5200,
			ops: []SetFreqOption{
				SetFreqChWidth(ChWidth160),
				SetFreqCenterFreq1(5250),
			},
			valid: true,
			args:  []string{"5200", "160", "5250"},
		},
		{
			ctrlFreq: 5240,
			ops:      []SetFreqOption{SetFreqChWidth(ChWidth80P80)},
			valid:    false,
		},
		{
			ctrlFreq: 5200,
			ops: []SetFreqOption{
				SetFreqChWidth(ChWidth80P80),
				SetFreqCenterFreq1(5210),
				SetFreqCenterFreq2(5530),
			},
			valid: true,
			args:  []string{"5200", "80+80", "5210", "5530"},
		},
	}

	for i, tc := range testcases {
		conf, err := newSetFreqConf(tc.ctrlFreq, tc.ops...)
		if !tc.valid {
			if err == nil {
				t.Errorf("testcase #%d should fail but succeed", i)
			}
			continue
		} else if err != nil {
			t.Errorf("testcase #%d failed with err=%s", i, err.Error())
		} else if args := conf.toArgs(); !reflect.DeepEqual(args, tc.args) {
			t.Errorf("testcase #%d failed, got args=%v, expect=%v", i, args, tc.args)
		}
	}
}

func TestExtractBSSID(t *testing.T) {
	const testStr = `Connected to 74:e5:43:10:4f:c0 (on wlan0)
	SSID: PMKSACaching_4m9p5_ch1
	freq: 5220
	RX: 5370 bytes (37 packets)
	TX: 3604 bytes (15 packets)
	signal: -59 dBm
	tx bitrate: 13.0 MBit/s MCS 1

	bss flags:      short-slot-time
	dtim period:    5
	beacon int:     100`
	expected := "74:e5:43:10:4f:c0"
	bss, err := extractBSSID(testStr)
	if err != nil {
		t.Errorf("unexpected error=%s", err.Error())
	} else if bss != expected {
		t.Errorf("got bss: %s, expect: %s", bss, expected)
	}
}

func TestRegulatoryDomain(t *testing.T) {
	testcases := []struct {
		out         string
		domain      string
		selfManaged bool
	}{
		// JP.
		{
			out: `global
country JP: DFS-JP
	(2402 - 2482 @ 40), (N/A, 20), (N/A)
	(2474 - 2494 @ 20), (N/A, 20), (N/A), NO-OFDM
	(4910 - 4990 @ 40), (N/A, 23), (N/A)
	(5030 - 5090 @ 40), (N/A, 23), (N/A)
	(5170 - 5250 @ 80), (N/A, 20), (N/A), AUTO-BW
	(5250 - 5330 @ 80), (N/A, 20), (0 ms), DFS, AUTO-BW
	(5490 - 5710 @ 160), (N/A, 23), (0 ms), DFS
	(59000 - 66000 @ 2160), (N/A, 10), (N/A)
`,
			domain:      "JP",
			selfManaged: false,
		},
		// US.
		{
			out: `global
country US: DFS-FCC
	(2402 - 2472 @ 40), (N/A, 30), (N/A)
	(5170 - 5250 @ 80), (N/A, 23), (N/A), AUTO-BW
	(5250 - 5330 @ 80), (N/A, 23), (0 ms), DFS, AUTO-BW
	(5490 - 5730 @ 160), (N/A, 23), (0 ms), DFS
	(5735 - 5835 @ 80), (N/A, 30), (N/A)
	(57240 - 71000 @ 2160), (N/A, 40), (N/A)
`,
			domain:      "US",
			selfManaged: false,
		},
		// Self managed.
		{
			out: `global
country 00: DFS-UNSET
	(2402 - 2472 @ 40), (N/A, 20), (N/A)
	(2457 - 2482 @ 20), (N/A, 20), (N/A), AUTO-BW, PASSIVE-SCAN
	(2474 - 2494 @ 20), (N/A, 20), (N/A), NO-OFDM, PASSIVE-SCAN
	(5170 - 5250 @ 80), (N/A, 20), (N/A), AUTO-BW, PASSIVE-SCAN
	(5250 - 5330 @ 80), (N/A, 20), (0 ms), DFS, AUTO-BW, PASSIVE-SCAN
	(5490 - 5730 @ 160), (N/A, 20), (0 ms), DFS, PASSIVE-SCAN
	(5735 - 5835 @ 80), (N/A, 20), (N/A), PASSIVE-SCAN
	(57240 - 63720 @ 2160), (N/A, 0), (N/A)

phy#0 (self-managed)
country US: DFS-UNSET
	(2402 - 2437 @ 40), (6, 22), (N/A), AUTO-BW, NO-HT40MINUS, NO-80MHZ, NO-160MHZ
	(2422 - 2462 @ 40), (6, 22), (N/A), AUTO-BW, NO-80MHZ, NO-160MHZ
	(2447 - 2482 @ 40), (6, 22), (N/A), AUTO-BW, NO-HT40PLUS, NO-80MHZ, NO-160MHZ
	(5170 - 5190 @ 80), (6, 22), (N/A), NO-OUTDOOR, AUTO-BW, IR-CONCURRENT, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5190 - 5210 @ 80), (6, 22), (N/A), NO-OUTDOOR, AUTO-BW, IR-CONCURRENT, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5210 - 5230 @ 80), (6, 22), (N/A), NO-OUTDOOR, AUTO-BW, IR-CONCURRENT, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5230 - 5250 @ 80), (6, 22), (N/A), NO-OUTDOOR, AUTO-BW, IR-CONCURRENT, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5250 - 5270 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5270 - 5290 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5290 - 5310 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5310 - 5330 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5490 - 5510 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5510 - 5530 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5530 - 5550 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5550 - 5570 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5570 - 5590 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5590 - 5610 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5610 - 5630 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5630 - 5650 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5650 - 5670 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5670 - 5690 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5690 - 5710 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5710 - 5730 @ 80), (6, 22), (0 ms), DFS, AUTO-BW, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5735 - 5755 @ 80), (6, 22), (N/A), AUTO-BW, IR-CONCURRENT, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5755 - 5775 @ 80), (6, 22), (N/A), AUTO-BW, IR-CONCURRENT, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5775 - 5795 @ 80), (6, 22), (N/A), AUTO-BW, IR-CONCURRENT, NO-HT40MINUS, NO-160MHZ, PASSIVE-SCAN
	(5795 - 5815 @ 80), (6, 22), (N/A), AUTO-BW, IR-CONCURRENT, NO-HT40PLUS, NO-160MHZ, PASSIVE-SCAN
	(5815 - 5835 @ 20), (6, 22), (N/A), AUTO-BW, IR-CONCURRENT, NO-HT40MINUS, NO-HT40PLUS, NO-80MHZ, NO-160MHZ, PASSIVE-SCAN
`,
			domain:      "00",
			selfManaged: true,
		},
	}

	mock := &stubCmdRunner{}
	r := &Runner{cmd: mock}
	for i, tc := range testcases {
		mock.out = []byte(tc.out)
		// Test regulatory domain.
		domain, err := r.RegulatoryDomain(context.Background())
		if err != nil {
			t.Errorf("case#%d, unexpected error in RegulatoryDomain: %v", i, err)
		} else if domain != tc.domain {
			t.Errorf("case#%d, got reg domain: %s, expect: %s", i, domain, tc.domain)
		}

		// Test self-managed with the same output of "iw reg get".
		selfManaged, err := r.IsRegulatorySelfManaged(context.Background())
		if err != nil {
			t.Errorf("case#%d, unexpected error in IsRegulatorySelfManaged: %v", i, err)
		} else if selfManaged != tc.selfManaged {
			t.Errorf("case#%d, got self managed: %t, expect: %t", i, selfManaged, tc.selfManaged)
		}
	}
}
