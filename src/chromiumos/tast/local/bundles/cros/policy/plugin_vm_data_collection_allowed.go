// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/plugin_vm_service_proto"
	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PluginVMDataCollectionAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of PluginVmDataCollectionAllowed policy",
		Contacts: []string{
			"okalitova@chromium.org", // Test author
		},
		SoftwareDeps: []string{"chrome", "plugin_vm"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.PluginVmDataCollectionAllowed{}, pci.VerifiedFunctionalityOS),
		},
	})
}

func PluginVMDataCollectionAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	const (
		dbusName      = "org.chromium.PluginVmService"
		dbusPath      = "/org/chromium/PluginVmService"
		dbusInterface = "org.chromium.PluginVmServiceInterface"
		pluginvmUID   = 20128 // See third_party/eclass-overlay/profiles/base/accounts/user/pluginvm.
	)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.PluginVmDataCollectionAllowed
	}{
		{
			name:  "true",
			value: &policy.PluginVmDataCollectionAllowed{Val: true},
		},
		{
			name:  "false",
			value: &policy.PluginVmDataCollectionAllowed{Val: false},
		},
		{
			name:  "unset",
			value: &policy.PluginVmDataCollectionAllowed{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.

			// We need to run the DBus methods as pluginvm. See "chrome/browser/chromeos/dbus/org.chromium.PluginVmService.conf".
			conn, obj, err := dbusutil.ConnectPrivateWithAuth(ctx, pluginvmUID, dbusName, dbus.ObjectPath(dbusPath))
			if err != nil {
				s.Fatalf("Failed to connect to %s: %v", dbusName, err)
			}
			defer conn.Close()

			var marshResponse []byte
			if err := obj.CallWithContext(ctx, dbusInterface+".GetPermissions", 0).Store(&marshResponse); err != nil {
				s.Fatal("Failed to get permissions: ", err)
			}
			response := new(cpb.GetPermissionsResponse)
			if err := proto.Unmarshal(marshResponse, response); err != nil {
				s.Fatal("Failed unmarshaling response: ", err)
			}
			isEnabled := response.GetDataCollectionEnabled()

			expectedEnabled := param.value.Stat != policy.StatusUnset && param.value.Val

			if isEnabled != expectedEnabled {
				s.Errorf("Unexpected enabled behavior: got %t; want %t", isEnabled, expectedEnabled)
			}
		})
	}
}
