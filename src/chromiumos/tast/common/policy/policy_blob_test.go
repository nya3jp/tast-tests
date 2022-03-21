// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	empb "chromiumos/policy/chromium/policy/enterprise_management_proto"
)

func TestMarshalBlob(t *testing.T) {
	deviceLocalAccountID := "foo"
	deviceLocalAccountType := AccountTypePublicSession

	tcs := []struct {
		pb     Blob
		result string
		isErr  bool
	}{
		{
			pb: Blob{
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
			result: `{"managed_users":["*"],"policies":[{"policy_type":"google/chromeos/user","value":"GgYSBGFzZGa6BQwSCgoDb25lCgN0d2/CEHcSdXsiaGFzaCI6ImJhZGRlY2FmYmFkZGVjYWZiYWRkZWNhZmJhZGRlY2FmYmFkZGVjYWZiYWRkZWNhZmJhZGRlY2FmYmFkZGVjYWYiLCJ1cmwiOiJodHRwczovL2V4YW1wbGUuY29tL3dhbGxwYXBlci5qcGcifboTAhAB"},{"policy_type":"google/chromeos/device","value":"igECIAGqAQkKBxIDZm9vGAA="},{"policy_type":"google/chromeos/publicaccount","entity_id":"id@managedchrome.com","value":"uhMCEAE="}],"policy_user":"tast-user@managedchrome.com"}`,
			isErr:  false,
		},
	}
	for _, tc := range tcs {
		b, err := tc.pb.MarshalJSON()
		if err != nil {
			if tc.isErr {
				continue
			}
			t.Errorf("Error marshalling the blob: %s", err)
		}
		if tc.isErr {
			t.Errorf("Expected the blob to fail to marshal, and got %v", string(b))
		}
		if string(b) != tc.result {
			t.Errorf("unexpected comparison between %s and %v", tc.result, string(b))
		}
	}
}

func TestUnmarshalBlob(t *testing.T) {
	deviceLocalAccountID := "foo"
	deviceLocalAccountType := AccountTypePublicSession

	tcs := []struct {
		result Blob
		json   string
		isErr  bool
	}{
		{
			result: Blob{
				ManagedUsers: []string{"*"},
				PolicyUser:   DefaultPolicyUser,
				DeviceProto: &empb.ChromeDeviceSettingsProto{
					AutoUpdateSettings: &empb.AutoUpdateSettingsProto{ScatterFactorInSeconds: &[]int64{1}[0]},
					DeviceLocalAccounts: &empb.DeviceLocalAccountsProto{
						Account: []*empb.DeviceLocalAccountInfoProto{{
							AccountId: &[]string{"foo"}[0],
							Type:      &[]empb.DeviceLocalAccountInfoProto_AccountType{empb.DeviceLocalAccountInfoProto_ACCOUNT_TYPE_PUBLIC_SESSION}[0],
						}},
					},
				},
				userProto: &empb.CloudPolicySettings{
					AllowDinosaurEasterEgg: &empb.BooleanPolicyProto{Value: &[]bool{true}[0]},
					DisabledSchemes:        &empb.StringListPolicyProto{Value: &empb.StringList{Entries: []string{"one", "two"}}},
					HomepageLocation:       &empb.StringPolicyProto{Value: &[]string{"asdf"}[0]},
					WallpaperImage:         &empb.StringPolicyProto{Value: &[]string{`{"hash":"baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf","url":"https://example.com/wallpaper.jpg"}`}[0]},
				},
				publicAccountsMapProto: map[string]*empb.CloudPolicySettings{
					"id@managedchrome.com": {
						AllowDinosaurEasterEgg: &empb.BooleanPolicyProto{Value: &[]bool{true}[0]},
					},
				},
				extensionPM:   make(BlobPolicyMap),
				RequestErrors: make(map[string]int),
				InitialState:  make(map[string]*BlobInitialState),
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
			json:  `{"managed_users":["*"],"policies":[{"policy_type":"google/chromeos/user","value":"GgYSBGFzZGa6BQwSCgoDb25lCgN0d2/CEHcSdXsiaGFzaCI6ImJhZGRlY2FmYmFkZGVjYWZiYWRkZWNhZmJhZGRlY2FmYmFkZGVjYWZiYWRkZWNhZmJhZGRlY2FmYmFkZGVjYWYiLCJ1cmwiOiJodHRwczovL2V4YW1wbGUuY29tL3dhbGxwYXBlci5qcGcifboTAhAB"},{"policy_type":"google/chromeos/device","value":"igECIAGqAQkKBxIDZm9vGAA="},{"policy_type":"google/chromeos/publicaccount","entity_id":"id@managedchrome.com","value":"uhMCEAE="}],"policy_user":"tast-user@managedchrome.com"}`,
			isErr: false,
		},
	}
	for _, tc := range tcs {
		var pb Blob
		err := pb.UnmarshalJSON([]byte(tc.json))
		if err != nil {
			if tc.isErr {
				continue
			}
			t.Errorf("Error unmarshalling the json into blob: %s", err)
		}
		if tc.isErr {
			t.Errorf("Expected the blob to fail to unmarshal, and got %v", pb)
		}
		if diff := cmp.Diff(pb.PolicyUser, tc.result.PolicyUser); diff != "" {
			t.Errorf("unexpected PolicyUser difference:\n%v", diff)
		}
		if diff := cmp.Diff(pb.ManagedUsers, tc.result.ManagedUsers); diff != "" {
			t.Errorf("unexpected PolicyUser difference:\n%v", diff)
		}
		if diff := cmp.Diff(pb.userProto, tc.result.userProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected userProto difference:\n%v", diff)
		}
		if diff := cmp.Diff(pb.DeviceProto, tc.result.DeviceProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected DeviceProto difference:\n%v", diff)
		}
		if diff := cmp.Diff(pb.publicAccountsMapProto, tc.result.publicAccountsMapProto, protocmp.Transform()); diff != "" {
			t.Errorf("unexpected publicAccountsMapProto difference:\n%v", diff)
		}
	}
}
