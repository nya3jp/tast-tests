// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type nl80211Attribute struct {
	name string
	// lowerBound and upperBound are inclusive
	lowerBound int64
	upperBound int64
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Nl80211StationInformation,
		Desc: "Verify the support for nl80211 station information",
		Contacts: []string{
			"kaidong@google.com",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827

		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		// Marvell chips don't support all the attributes
		HardwareDeps: hwdep.D(hwdep.WifiNotMarvell()),
		Fixture:      "wificellFixt",
		Timeout:      2 * time.Minute,
	})
}

/*
nl80211 is the 802.11 netlink interface public header. It provides station
information of an interface, such as "rx bytes", "tx bytes", "rx packets", "tx
packets".

This test uses the iw tool to get the nl80211 station information of the WiFi
interface, and checks if the DUT supports all nl80211 attributes required by the
WiFi link statistics feature.

Attribute in nl80211_sta_info           Label in the iw output      Data type
NL80211_STA_INFO_RX_BYTES               rx bytes                    u32
NL80211_STA_INFO_RX_PACKETS             rx packets                  u32
NL80211_STA_INFO_TX_BYTES               tx bytes                    u32
NL80211_STA_INFO_TX_PACKETS             tx packets                  u32
NL80211_STA_INFO_TX_RETRIES             tx retries                  u32
NL80211_STA_INFO_TX_FAILED              tx failed                   u32
NL80211_STA_INFO_RX_DROP_MISC           rx drop misc                u64
NL80211_STA_INFO_SIGNAL_AVG             signal avg                  u8
NL80211_STA_INFO_SIGNAL                 signal                      u8
*/

const (
	nl80211RxBytes          = "rx bytes"
	nl80211TxBytes          = "tx bytes"
	nl80211RxPackets        = "rx packets"
	nl80211TxPackets        = "tx packets"
	nl80211RxDrop           = "rx drop misc"
	nl80211TxRetries        = "tx retries"
	nl80211TxFailed         = "tx failed"
	nl80211SignalAvg        = "signal avg"
	nl80211Signal           = "signal"
	nl80211SignalLowerBound = -100
	nl80211SignalUpperBound = 0
)

func Nl80211StationInformation(ctx context.Context, s *testing.State) {

	// nl80211 station information attributes required by the WiFi link
	// statistics feature.
	// Some devices support NL80211_STA_INFO_RX_BYTES64 and
	// NL80211_STA_INFO_TX_BYTES64, then "rx bytes" and "tx bytes" are
	// represented by u64.
	nl80211Attributes := []nl80211Attribute{
		{nl80211RxBytes, 1, math.MaxInt64},
		{nl80211TxBytes, 1, math.MaxInt64},
		{nl80211RxPackets, 1, math.MaxUint32},
		{nl80211TxPackets, 1, math.MaxUint32},
		{nl80211RxDrop, 0, math.MaxInt64},
		{nl80211TxRetries, 0, math.MaxUint32},
		{nl80211TxFailed, 0, math.MaxUint32},
		{nl80211SignalAvg, nl80211SignalLowerBound, nl80211SignalUpperBound},
		{nl80211Signal, nl80211SignalLowerBound, nl80211SignalUpperBound}}

	tf := s.FixtValue().(*wificell.TestFixture)

	apOpts := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}
	apIface, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Log("AP setup done")

	_, err = tf.ConnectWifiAP(ctx, apIface)
	if err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	// Get "iw dev interface station dump" output
	iwr := iw.NewRemoteRunner(s.DUT().Conn())
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface, err: ", err)
	}
	stationInformation, err := iwr.AllStationInformation(ctx, clientIface)
	if err != nil {
		s.Fatal("Failed to get the nl80211 station information, err: ", err)
	}

	attributeValues := make(map[string]int64)
	// Check if all required attributes exist in the output
	s.Log("The nl80211 station information is")
	for _, attribute := range nl80211Attributes {
		output, ok := stationInformation[attribute.name]
		if !ok {
			s.Errorf("nl80211_sta_info attribute %q is missing", attribute.name)
			continue
		}
		value, err := parseValue(attribute.name, output)
		// Check if the values of attributes fall in the range
		if err != nil || value < attribute.lowerBound || value > attribute.upperBound {
			s.Errorf("nl80211_sta_info attribute %q has invalid value: %s", attribute.name, output)
			continue
		}
		attributeValues[attribute.name] = value
		s.Logf("%s: %s", attribute.name, output)
	}
}

func parseValue(name, text string) (value int64, err error) {
	number := text
	// An example of the signal output, the values in the brackets are signal
	// strength per chain
	// signal:              -56 [-56, -59] dBm
	// signal avg:          -60 dBm
	if name == nl80211Signal || name == nl80211SignalAvg {
		numbers := strings.Split(text, " ")
		if len(numbers) == 0 {
			return value, errors.New("failed to get the number")
		}
		number = numbers[0]
	}
	return strconv.ParseInt(number, 10, 64)
}
