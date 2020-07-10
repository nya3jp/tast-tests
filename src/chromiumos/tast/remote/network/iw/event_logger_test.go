// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iw

import (
	"math"
	"testing"
	"time"
)

func TestParseEvent(t *testing.T) {
	testcases := []struct {
		input    string
		expected *Event
	}{
		{
			input: "1578622653.543296: wlan0: new station aa:bb:cc:dd:ee:ff",
			expected: &Event{
				Type:      EventTypeUnknown,
				Timestamp: time.Unix(1578622653, 543296000),
				Interface: "wlan0",
				Message:   "new station aa:bb:cc:dd:ee:ff",
			},
		},
		{
			input: "1578622656.590595: wlan0 (phy #5): assoc aa:bb:cc:dd:ee:ff -> 1a:2b:3c:4d:5e:6f status: 0: Successful",
			expected: &Event{
				Type:      EventTypeUnknown,
				Timestamp: time.Unix(1578622656, 590595000),
				Interface: "wlan0",
				Message:   "assoc aa:bb:cc:dd:ee:ff -> 1a:2b:3c:4d:5e:6f status: 0: Successful",
			},
		},
		// Handcrafted testcase to test wdev case.
		{
			input: "1.000005: wdev 0x1: message",
			expected: &Event{
				Type:      EventTypeUnknown,
				Timestamp: time.Unix(1, 5),
				Interface: "wdev 0x1",
				Message:   "message",
			},
		},
		// Testcases for different EventType.
		{
			input: "1.000000: wlan0 (phy #13): disconnected (local request) reason: 3: Deauthenticated because sending station is leaving (or has left) the IBSS or ESS",
			expected: &Event{
				Type:      EventTypeDisconnect,
				Timestamp: time.Unix(1, 0),
				Interface: "wlan0",
				Message:   "disconnected (local request) reason: 3: Deauthenticated because sending station is leaving (or has left) the IBSS or ESS",
			},
		},
		{
			input: "1594348712.637469: wlan0 (phy #0): scan started",
			expected: &Event{
				Type:      EventTypeScanStart,
				Timestamp: time.Unix(1594348712, 637469000),
				Interface: "wlan0",
				Message:   "scan started",
			},
		},
		{
			input: "1594348724.592450: wlan0 (phy #0): connected to 3c:28:6d:c4:79:fc",
			expected: &Event{
				Type:      EventTypeConnected,
				Timestamp: time.Unix(1594348724, 592450000),
				Interface: "wlan0",
				Message:   "connected to 3c:28:6d:c4:79:fc",
			},
		},
	}

	// We may have float parsing error, so construct our own compare.
	compare := func(actual, want *Event) bool {
		if (actual == nil) != (want == nil) {
			return false
		}
		if actual == nil {
			return true
		}
		if actual.Type != want.Type {
			return false
		}
		if actual.Interface != want.Interface {
			return false
		}
		if actual.Message != want.Message {
			return false
		}
		const accuracyInMilliseconds = 1e6 // Let's check accuracy to millisecond (i.e. 1e6 nano).
		if math.Abs(float64(actual.Timestamp.UnixNano()-want.Timestamp.UnixNano())) > accuracyInMilliseconds {
			return false
		}
		return true
	}

	for i, tc := range testcases {
		ev, err := parseEvent(tc.input)
		if err != nil {
			t.Errorf("error on case %d: err=%s", i, err.Error())
		} else if !compare(ev, tc.expected) {
			t.Errorf("error on case %d: got %v, want %v", i, ev, tc.expected)
		}
	}
}
