// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	empb "chromiumos/policy/chromium/policy/enterprise_management_proto"
)

func TestMarshalAndUnmarshalBlob(t *testing.T) {
	deviceLocalAccountID := "foo"
	deviceLocalAccountType := AccountTypePublicSession

	tcs := []struct {
		srcBlob Blob
		name    string
	}{
		{
			name: "user policy",
			srcBlob: Blob{
				ManagedUsers:           []string{"*"},
				PolicyUser:             DefaultPolicyUser,
				DeviceProto:            &empb.ChromeDeviceSettingsProto{},
				userProto:              &empb.CloudPolicySettings{},
				publicAccountsMapProto: make(map[string]*empb.CloudPolicySettings),
				extensionPM:            make(BlobPolicyMap),
				RequestErrors:          make(map[string]int),
				InitialState:           make(map[string]*BlobInitialState),
				userPolicies: []Policy{
					&AllowDinosaurEasterEgg{Val: true},
					&DisabledSchemes{Val: []string{"one", "two"}},
					&HomepageLocation{Val: "asdf"},
					&WallpaperImage{
						Val: &WallpaperImageValue{
							Url:  "https://example.com/wallpaper.jpg",
							Hash: "baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf",
						},
					},
				},
			},
		},
		{
			name: "device policy",
			srcBlob: Blob{
				ManagedUsers:           []string{"*"},
				PolicyUser:             DefaultPolicyUser,
				DeviceProto:            &empb.ChromeDeviceSettingsProto{},
				userProto:              &empb.CloudPolicySettings{},
				publicAccountsMapProto: make(map[string]*empb.CloudPolicySettings),
				extensionPM:            make(BlobPolicyMap),
				RequestErrors:          make(map[string]int),
				InitialState:           make(map[string]*BlobInitialState),
				devicePolicies: []Policy{
					&DeviceUpdateScatterFactor{Val: 1},
				},
			},
		},
		{
			name: "public account policy",
			srcBlob: Blob{
				ManagedUsers:           []string{"*"},
				PolicyUser:             DefaultPolicyUser,
				DeviceProto:            &empb.ChromeDeviceSettingsProto{},
				userProto:              &empb.CloudPolicySettings{},
				publicAccountsMapProto: make(map[string]*empb.CloudPolicySettings),
				extensionPM:            make(BlobPolicyMap),
				RequestErrors:          make(map[string]int),
				InitialState:           make(map[string]*BlobInitialState),
				devicePolicies: []Policy{
					&DeviceLocalAccounts{
						Val: []DeviceLocalAccountInfo{
							{
								AccountID:   &deviceLocalAccountID,
								AccountType: &deviceLocalAccountType,
							},
						},
					},
				},
				publicAccountPolicies: map[string][]Policy{
					"id@managedchrome.com": {&AllowDinosaurEasterEgg{Val: true}},
				},
			},
		},
		{
			name: "extension policy",
			srcBlob: Blob{
				ManagedUsers:           []string{"*"},
				PolicyUser:             DefaultPolicyUser,
				DeviceProto:            &empb.ChromeDeviceSettingsProto{},
				userProto:              &empb.CloudPolicySettings{},
				publicAccountsMapProto: make(map[string]*empb.CloudPolicySettings),
				extensionPM: BlobPolicyMap{
					"ibdnofdagboejmpijdiknapcihkomkki": json.RawMessage([]byte(`{
						"VisibleStringPolicy": {
							"Value": "notsecret"
						},
						"SensitiveStringPolicy": {
							"Value": "secret"
						},
						"VisibleDictPolicy": {
						  "Value": {
							"some_bool": true,
							"some_string": "notsecret"
						  }
						},
						"SensitiveDictPolicy": {
							"Value": {
								"some_bool": true,
								"some_string": "secret"
							}
						}
					}`)),
				},
				RequestErrors: make(map[string]int),
				InitialState:  make(map[string]*BlobInitialState),
			},
		},
	}
	for _, tc := range tcs {
		b, err := tc.srcBlob.MarshalJSON()
		if err != nil {
			t.Fatalf("Error marshalling the blob for subtest %s: %s", tc.name, err)
		}
		var resultBlob Blob
		resultBlob.extensionPM = make(BlobPolicyMap)
		if err := resultBlob.UnmarshalJSON(b); err != nil {
			t.Fatalf("Error unmarshalling the raw json %s into the blob for subtest %s: %s", string(b), tc.name, err)
		}
		if diff := cmp.Diff(resultBlob.PolicyUser, tc.srcBlob.PolicyUser); diff != "" {
			t.Errorf("unexpected PolicyUser difference for subtest %s:\n%v", tc.name, diff)
		}
		if diff := cmp.Diff(resultBlob.ManagedUsers, tc.srcBlob.ManagedUsers); diff != "" {
			t.Errorf("unexpected ManagedUsers difference for subtest %s:\n%v", tc.name, diff)
		}
		if diff := cmp.Diff(resultBlob.userProto, tc.srcBlob.userProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected userProto difference for subtest %s:\n%v", tc.name, diff)
		}
		if diff := cmp.Diff(resultBlob.DeviceProto, tc.srcBlob.DeviceProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected DeviceProto difference for subtest %s:\n%v", tc.name, diff)
		}
		if diff := cmp.Diff(resultBlob.publicAccountsMapProto, tc.srcBlob.publicAccountsMapProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected publicAccountsMapProto difference for subtest %s:\n%v", tc.name, diff)
		}
		if diff := cmp.Diff(resultBlob.extensionPM, tc.srcBlob.extensionPM); diff != "" {
			t.Errorf("unexpected extensionPM difference for subtest %s:\n%v", tc.name, diff)
		}
	}
}
