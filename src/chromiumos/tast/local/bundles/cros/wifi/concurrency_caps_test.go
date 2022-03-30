// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"testing"

	"chromiumos/tast/common/network/iw"
)

func TestSupportsConcurrency(t *testing.T) {
	for i, tc := range []struct {
		combination      iw.IfaceCombination
		concurrentIfaces []string
		expectedResult   bool
	}{
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"AP",
							"P2P-client",
							"P2P-GO",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"P2P-device",
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]string{"managed", "AP"},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"AP",
							"P2P-client",
							"P2P-GO",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"P2P-device",
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]string{"managed", "AP", "P2P-client", "P2P-device"},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"AP",
							"P2P-client",
							"P2P-GO",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"P2P-device",
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]string{"managed", "managed", "AP", "P2P-client", "P2P-device"},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"AP",
							"P2P-client",
							"P2P-GO",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"P2P-device",
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]string{"invalid-name"},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"P2P-client",
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []string{
							"AP",
							"P2P-GO",
						},
						MaxCount: 1,
					},
					{
						IfaceTypes: []string{
							"P2P-device",
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 2,
			},
			[]string{"managed", "AP"},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 0,
					},
				},
				MaxTotal:    2,
				MaxChannels: 1,
			},
			[]string{"managed"},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []string{
							"managed",
						},
						MaxCount: 3,
					},
				},
				MaxTotal:    3,
				MaxChannels: 1,
			},
			[]string{"managed", "managed", "managed"},
			true,
		},
	} {
		result := supportsConcurrency(tc.combination, tc.concurrentIfaces)
		if tc.expectedResult != result {
			t.Errorf("testcase %d failed; got %t; want %t", i, result, tc.expectedResult)
		}

	}
}
