// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
			"kaidong@google.com",
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		// Marvell chips don't support all the attributes
		HardwareDeps: hwdep.D(hwdep.WifiNotMarvell()),
		Fixture:      "wificellFixt",
		Timeout:      2 * time.Minute,
	})
}

/*
nl80211 is the new 802.11 netlink interface public header. It provides station
information of an interface, such as "rx bytes", "tx bytes", "rx packets", "tx
packets".

This test uses the iw tool https://source.corp.google.com/eureka_internal/external/iw/station.c;l=291
to get the nl80211 station information of the WiFi interface, and checks if the
DUT supports all nl80211 attributes required by the WiFi link statistics feature.

Attribute in nl80211_sta_info           Label in the iw output      Data type
NL80211_STA_INFO_RX_BYTES               rx bytes                    u32
NL80211_STA_INFO_RX_PACKETS             rx packets                  u32
NL80211_STA_INFO_TX_BYTES               tx bytes                    u32
NL80211_STA_INFO_TX_PACKETS             tx packets                  u32
NL80211_STA_INFO_TX_RETRIES             tx retries                  u32
NL80211_STA_INFO_TX_FAILED              tx failed                   u32
NL80211_STA_INFO_RX_DROP_MISC           rx drop misc                u64
NL80211_STA_INFO_BEACON_SIGNAL_AVG      beacon signal avg           u8
NL80211_STA_INFO_SIGNAL_AVG             signal avg                  u8
NL80211_STA_INFO_SIGNAL                 signal                      u8
*/

func Nl80211StationInformation(ctx context.Context, s *testing.State) {
	const (
		// The difference between NL80211_STA_INFO_SIGNAL_AVG,
		// NL80211_STA_INFO_SIGNAL and NL80211_STA_INFO_BEACON_SIGNAL_AVG. This
		// value is subject to changes based on test results.
		signalError      = 6
		signalLowerBound = -100
		signalUpperBound = -25
	)
	// nl80211 station information attributes required by the WiFi link
	// statistics feature.
	// Some devices support NL80211_STA_INFO_RX_BYTES64 and
	// NL80211_STA_INFO_TX_BYTES64, then "rx bytes" and "tx bytes" are
	// represented by u64.
	nl80211Attributes := []nl80211Attribute{
		{"rx bytes", 1, math.MaxInt64},
		{"tx bytes", 1, math.MaxInt64},
		{"rx packets", 1, math.MaxUint32},
		{"tx packets", 1, math.MaxUint32},
		{"rx drop misc", 0, math.MaxInt64},
		{"tx retries", 0, math.MaxUint32},
		{"tx failed", 0, math.MaxUint32},
		{"beacon signal avg", signalLowerBound, signalUpperBound},
		{"signal avg", signalLowerBound, signalUpperBound},
		{"signal", signalLowerBound, signalUpperBound}}

	tf := s.FixtValue().(*wificell.TestFixture)

	apOpts := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()
	apIface, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(cleanupCtx)
	s.Log("AP setup done")

	_, err = tf.ConnectWifiAP(ctx, apIface)
	if err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(cleanupCtx)
	s.Log("Connected")

	// Get "iw dev interface station dump" output
	iwr := iw.NewRemoteRunner(s.DUT().Conn())
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface, err: ", err)
	}
	kvs, err := iwr.AllStationInformation(ctx, clientIface)
	if err != nil {
		s.Fatal("Failed to get the nl80211 station information, err: ", err)
	}

	var missingAttributes []string
	attributeValues := make(map[string]int64)
	invalidMessage := ""
	invalidCount := 0

	// Check if all required attributes exist in the output
	s.Log("The nl80211 station information is")
	for _, attribute := range nl80211Attributes {
		output, ok := kvs[attribute.name]
		if !ok {
			missingAttributes = append(missingAttributes, attribute.name)
			continue
		}
		s.Logf("%s: %s", attribute.name, output)
		value, err := parseValue(attribute.name, output)
		if err == nil {
			attributeValues[attribute.name] = value
		} else {
			invalidMessage += fmt.Sprintf("%s %s, ", attribute.name, output)
			invalidCount++
		}
	}
	if missingAttributes != nil {
		s.Fatalf("%d nl80211_sta_info attributes are missing: %q", len(missingAttributes), missingAttributes)
	}

	// Check if the values of attributes fall in the range
	for _, attribute := range nl80211Attributes {
		if value, ok := attributeValues[attribute.name]; ok && (value < attribute.lowerBound || value > attribute.upperBound) {
			invalidMessage += fmt.Sprintf("%s %d, ", attribute.name, value)
			invalidCount++
		}
	}
	if invalidCount > 0 {
		s.Fatalf("%d nl80211_sta_info attributes have invalid values: %s", invalidCount, invalidMessage)
	}

	// Check if the difference between signal, signal avg and beacon signal avg
	// is too large
	for _, attribute := range []string{"signal", "signal avg"} {
		if math.Abs(float64(attributeValues[attribute]-attributeValues["beacon signal avg"])) > signalError {
			s.Fatalf("The difference between %s %d and beacon signal avg %d is larger than the threshold %d", attribute, attributeValues[attribute], attributeValues["beacon signal avg"], signalError)
		}
	}
}

func parseValue(name, text string) (value int64, err error) {
	number := text
	// An example of the signal output, the values in the brackets are signal
	// strength per chain
	// signal:              -56 [-56, -59] dBm
	// signal avg:          -60 dBm
	// beacon signal avg:   -58 dBm
	if name == "signal" || name == "signal avg" || name == "beacon signal avg" {
		numbers := strings.Split(text, " ")
		if len(numbers) == 0 {
			return value, errors.New("failed to get the number")
		}
		number = numbers[0]
	}
	value, err = strconv.ParseInt(number, 10, 64)
	return value, err
}
