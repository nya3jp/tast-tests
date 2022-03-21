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

	tcs := []Blob{
		{
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
			devicePolicies: []Policy{
				&DeviceUpdateScatterFactor{Val: 1},
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
		{
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
	}
	for _, srcBlob := range tcs {
		b, err := srcBlob.MarshalJSON()
		if err != nil {
			t.Errorf("Error marshalling the blob: %s", err)
		}
		var resultBlob Blob
		if err := resultBlob.UnmarshalJSON(b); err != nil {
			t.Errorf("Error unmarshalling the raw json %s into the blob: %s", string(b), err)
		}
		if diff := cmp.Diff(resultBlob.PolicyUser, srcBlob.PolicyUser); diff != "" {
			t.Errorf("unexpected PolicyUser difference:\n%v", diff)
		}
		if diff := cmp.Diff(resultBlob.ManagedUsers, srcBlob.ManagedUsers); diff != "" {
			t.Errorf("unexpected ManagedUsers difference:\n%v", diff)
		}
		if diff := cmp.Diff(resultBlob.userProto, srcBlob.userProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected userProto difference:\n%v", diff)
		}
		if diff := cmp.Diff(resultBlob.DeviceProto, srcBlob.DeviceProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected DeviceProto difference:\n%v", diff)
		}
		if diff := cmp.Diff(resultBlob.publicAccountsMapProto, srcBlob.publicAccountsMapProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected publicAccountsMapProto difference:\n%v", diff)
		}
		if srcBlob.extensionPM != nil && len(srcBlob.extensionPM) > 0 {
			if diff := cmp.Diff(resultBlob.extensionPM, srcBlob.extensionPM); diff != "" {
				t.Errorf("unexpected extensionPM difference:\n%v", diff)
			}
		}
	}
}
