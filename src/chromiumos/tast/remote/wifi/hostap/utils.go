// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostap

import "chromiumos/tast/errors"

// freqToChannelMap map frequenty (MHz) to channel number (ported from Autotest).
var freqToChannelMap = map[int]int{
	2412: 1,
	2417: 2,
	2422: 3,
	2427: 4,
	2432: 5,
	2437: 6,
	2442: 7,
	2447: 8,
	2452: 9,
	2457: 10,
	2462: 11,
	// 12, 13 are only legitimate outside the US.
	2467: 12,
	2472: 13,
	// 14 is for Japan, DSSS and CCK only.
	2484: 14,
	// 32 valid in Europe.
	5160: 32,
	// 34 valid in Europe.
	5170: 34,
	// 36-116 valid in the US, except 38, 42, and 46, which have
	// mixed international support.
	5180: 36,
	5190: 38,
	5200: 40,
	5210: 42,
	5220: 44,
	5230: 46,
	5240: 48,
	5260: 52,
	5280: 56,
	5300: 60,
	5320: 64,
	5500: 100,
	5520: 104,
	5540: 108,
	5560: 112,
	5580: 116,
	// 120, 124, 128 valid in Europe/Japan.
	5600: 120,
	5620: 124,
	5640: 128,
	// 132+ valid in US.
	5660: 132,
	5680: 136,
	5700: 140,
	5710: 142,
	// 144 is supported by a subset of WiFi chips
	// (e.g. bcm4354, but not ath9k).
	5720: 144,
	5745: 149,
	5755: 151,
	5765: 153,
	5785: 157,
	5805: 161,
	5825: 165,
}

// FrequencyToChannel maps center frequency (in MHz) to the corresponding channel.
func FrequencyToChannel(freq int) (int, error) {
	ch, ok := freqToChannelMap[freq]
	if !ok {
		return 0, errors.Errorf("cannot find channel with frequency=%d", freq)
	}
	return ch, nil
}

// ChannelToFrequency maps channel id to its center frequency (in MHz).
func ChannelToFrequency(target int) (int, error) {
	var freq = 0
	for f, ch := range freqToChannelMap {
		if ch == target {
			freq = f
			break
		}
	}
	if freq == 0 {
		return 0, errors.Errorf("cannnot find channel num=%d", target)
	}
	return freq, nil
}
