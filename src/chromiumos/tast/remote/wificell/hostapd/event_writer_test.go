// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEventWriter(t *testing.T) {
	// Test event writer with deauth logs.
	testcases := []struct {
		buf       []byte
		write     []byte
		remainBuf []byte
		events    []*Event
	}{
		{
			buf: nil,
			write: []byte(`1586321757.108516: Allowed channel: mode=2 chan=161 freq=5805 MHz max_tx_power=23 dBm
1586321757.108537: Allowed channel: mode=2 chan=165 freq=5825 MHz max_tx_power=23 dBm
1586321757.108561: Completing interface initialization
158632`),
			remainBuf: []byte("158632"),
			events:    nil,
		},
		{
			// Test buffer append.
			buf:       []byte("15863217"),
			write:     []byte("57.108516: Allowed ch"),
			remainBuf: []byte("1586321757.108516: Allowed ch"),
			events:    nil,
		},
		{
			// Test deauth event detection.
			buf: nil,
			write: []byte(`1586321765.799931: managed0: Event RX_MGMT (19) received
1586321765.799964: managed0: mgmt::deauth
1586321765.800001: managed0: deauthentication: STA=00:1a:2b:3c:4d:5e reason_code=3
1586321765.800038: managed0: AP-STA-DISCONNECTED 00:1a:2b:3c:4d:5e
`),
			remainBuf: nil,
			events: []*Event{
				&Event{
					Type:   EventDeauth,
					Client: "00:1a:2b:3c:4d:5e",
					Msg:    "1586321765.800001: managed0: deauthentication: STA=00:1a:2b:3c:4d:5e reason_code=3",
				},
			},
		},
		{
			// Test event detection with buffer.
			buf:       []byte("1586321765.800001: managed0: deauthentic"),
			write:     []byte("ation: STA=00:1a:2b:3c:4d:5e reason_code=3\n"),
			remainBuf: nil,
			events: []*Event{
				&Event{
					Type:   EventDeauth,
					Client: "00:1a:2b:3c:4d:5e",
					Msg:    "1586321765.800001: managed0: deauthentication: STA=00:1a:2b:3c:4d:5e reason_code=3",
				},
			},
		},
	}

	writer := NewEventWriter()
	for i, tc := range testcases {
		writer.buf = tc.buf
		writer.events = nil
		written, err := writer.Write(tc.write)
		if err != nil {
			t.Errorf("testcase#%d: failed with err=%v", i, err)
			continue
		}
		if written != len(tc.write) {
			t.Errorf("testcase#%d: Write returns %d, want %d", i, written, len(tc.write))
		}
		if !bytes.Equal(writer.buf, tc.remainBuf) {
			t.Errorf("testcase#%d: writer has remaining buffer %q, want %q", i, string(writer.buf), string(tc.remainBuf))
		}
		if !reflect.DeepEqual(writer.Events(), tc.events) {
			t.Errorf("testcase#%d: writer detects events %v, want %v", i, writer.Events(), tc.events)
		}
	}
}
