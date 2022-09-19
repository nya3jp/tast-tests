// Copyright 2022 The ChromiumOS Authors
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
		concurrentIfaces []iw.IfType
		expectedResult   bool
	}{
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeAP,
							iw.IfTypeP2PClient,
							iw.IfTypeP2PGO,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeManaged, iw.IfTypeAP},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeAP,
							iw.IfTypeP2PClient,
							iw.IfTypeP2PGO,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeManaged, iw.IfTypeAP, iw.IfTypeP2PClient},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeAP,
							iw.IfTypeP2PClient,
							iw.IfTypeP2PGO,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeManaged, iw.IfTypeManaged, iw.IfTypeAP, iw.IfTypeP2PClient, iw.IfTypeP2PDevice},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeAP,
							iw.IfTypeP2PClient,
							iw.IfTypeP2PGO,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfType("invalid-name")},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PClient,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeAP,
							iw.IfTypeP2PGO,
						},
						MaxCount: 1,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    4,
				MaxChannels: 2,
			},
			[]iw.IfType{iw.IfTypeManaged, iw.IfTypeAP},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 0,
					},
				},
				MaxTotal:    2,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeManaged},
			false,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeManaged,
						},
						MaxCount: 3,
					},
				},
				MaxTotal:    3,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeManaged, iw.IfTypeManaged, iw.IfTypeManaged},
			true,
		},
		{
			iw.IfaceCombination{
				IfaceLimits: []iw.IfaceLimit{
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PGO,
							iw.IfTypeP2PClient,
						},
						MaxCount: 2,
					},
					{
						IfaceTypes: []iw.IfType{
							iw.IfTypeP2PDevice,
						},
						MaxCount: 1,
					},
				},
				MaxTotal:    3,
				MaxChannels: 1,
			},
			[]iw.IfType{iw.IfTypeP2PGO, iw.IfTypeP2PClient},
			true,
		},
	} {
		result := supportsConcurrency(tc.combination, tc.concurrentIfaces)
		if tc.expectedResult != result {
			t.Errorf("testcase %d failed; got %t; want %t", i, result, tc.expectedResult)
		}

	}
}
