// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
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

	// Read file content to a buffer for reuse.
	buf, err := ioutil.ReadFile(testfile)
	if err != nil {
		t.Fatalf("failed to read test pcap file: %v", err)
	}

	// Read all packets as reference answer.
	refPackets, err := ReadPacketsFromReader(bytes.NewBuffer(buf))
	if err != nil {
		t.Fatalf("failed to read all packets: %v", err)
	}

caseLoop:
	for _, tc := range testcases {
		ret, err := ReadPacketsFromReader(bytes.NewBuffer(buf), tc.filters...)
		if err != nil {
			t.Errorf("Case %q: failed to packets: %v", tc.name, err)
			continue
		}
		if len(ret) != len(tc.resIndices) {
			t.Errorf("Case %q: unexpected length of results: got %d, want %d", tc.name, len(ret), len(tc.resIndices))
			continue
		}
		for i, idx := range tc.resIndices {
			if idx < 0 || idx >= len(refPackets) {
				t.Errorf("Case %q: bug in testcase: out of range ID=%d", tc.name, idx)
				continue caseLoop
			}
			if !reflect.DeepEqual(ret[i], refPackets[idx]) {
				t.Errorf("Case %q: unexpected %d-th packet: got %v, want %v", tc.name, i, ret[i], refPackets[idx])
				continue caseLoop
			}
		}
	}
}
