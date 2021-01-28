// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/plugin_vm_service_proto"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PluginVmDataCollectionAllowed,
		Desc: "Behavior of PluginVmDataCollectionAllowed policy",
		Contacts: []string{
			"okalitova@chromium.org", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome", "plugin_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func PluginVmDataCollectionAllowed(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

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
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
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
