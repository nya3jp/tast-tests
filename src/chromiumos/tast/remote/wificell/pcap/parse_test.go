// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"reflect"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/errors"
)

func TestReadPackets(t *testing.T) {
	// Test with one pcap file from RandomMACAddr test.
	testfile := "testdata/random_mac_addr.pcap"

	testcases := []struct {
		name    string
		filters []Filter
		// The indices of packets in the file that will pass the filter.
		// As it is quite tedious to construct every layers of the result
		// packets to compare, we test the filter with the indices of
		// returned packets in the file instead.
		// This can be obtained by applying the similar filter in Wireshark.
		// Notice: IDs in Wireshark start from 1, but array indices start
		// from 0 and we are using the array indices here.
		resIndices []int
	}{
		{
			name: "probe_req",
			filters: []Filter{
				RejectLowSignal(),
				Dot11FCSValid(),
				TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil),
			},
			resIndices: []int{9, 13, 16, 17, 72, 76, 79, 132, 136, 187,
				191, 194, 200, 252, 256},
		},
		{
			name: "probe_resp_to_addr",
			filters: []Filter{
				TypeFilter(layers.LayerTypeDot11MgmtProbeResp, nil),
				TypeFilter(layers.LayerTypeDot11,
					func(layer gopacket.Layer) bool {
						// Filter the receiver.
						dot11 := layer.(*layers.Dot11)
						return dot11.Address1.String() == "dc:53:60:01:94:7d"
					}),
			},
			resIndices: []int{10, 14, 18, 19, 20, 21},
		},
	}

	// Read all packets as reference answer.
	refPackets, err := ReadPackets(testfile)
	if err != nil {
		t.Fatalf("failed to read all packets: %v", err)
	}

	// Function for checking if the result is as expected.
	checkResult := func(got []gopacket.Packet, expectIndices []int) error {
		if len(got) != len(expectIndices) {
			return errors.Errorf("unexpected length of results: got %d, want %d", len(got), len(expectIndices))
		}
		for i, idx := range expectIndices {
			if idx < 0 || idx >= len(refPackets) {
				return errors.Errorf("bug in testcase: out of range packet index=%d", idx)
			}
			if !reflect.DeepEqual(got[i], refPackets[idx]) {
				return errors.Errorf("unexpected %d-th packet: got %v, want %v", i, got[i], refPackets[idx])
			}
		}
		return nil
	}

	for _, tc := range testcases {
		ret, err := ReadPackets(testfile, tc.filters...)
		if err != nil {
			t.Errorf("Case %q: failed to read packets: %v", tc.name, err)
			continue
		}
		if err := checkResult(ret, tc.resIndices); err != nil {
			t.Errorf("Case %q: %v", tc.name, err)
		}
	}
}
