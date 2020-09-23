// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ArcBackupRestoreServiceEnabled,
		Desc: "Test the behavior of ArcBackupRestoreServiceEnabled policy: check the Backup Manager state after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 8 * time.Minute,
	})
}

// ArcBackupRestoreServiceEnabled tests the ArcBackupRestoreServiceEnabled policy.
func ArcBackupRestoreServiceEnabled(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	pb := fakedms.NewPolicyBlob()

	for _, param := range []struct {
		name        string
		wantEnabled bool
		value       *policy.ArcBackupRestoreServiceEnabled
	}{
		{
			name:        "disabled",
			wantEnabled: false,
			value:       &policy.ArcBackupRestoreServiceEnabled{Val: 0},
		},
		{
			name:        "user_decides",
			wantEnabled: false,
			value:       &policy.ArcBackupRestoreServiceEnabled{Val: 1},
		},
		{
			name:        "unset_1",
			wantEnabled: false,
			value:       &policy.ArcBackupRestoreServiceEnabled{Stat: policy.StatusUnset},
		},
		{
			name:        "enabled",
			wantEnabled: true,
			value:       &policy.ArcBackupRestoreServiceEnabled{Val: 2},
		},
		{
			name:        "unset_2",
			wantEnabled: false,
			value:       &policy.ArcBackupRestoreServiceEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policy blob.
			pb.AddPolicies([]policy.Policy{param.value})
			if err := fdms.WritePolicyBlob(pb); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}

			// Start a Chrome instance that will fetch policies from the FakeDMS.
			// This policy must be updated before starting Chrome.
			cr, err := chrome.New(ctx,
				chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
				chrome.DMSPolicy(fdms.URL),
				chrome.ARCEnabled())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)

			a, err := arc.New(ctx, s.OutDir())
			if err != nil {
				s.Fatal("Failed to start ARC: ", err)
			}
			defer a.Close()

			// Get ARC Backup Manager state.
			var enabled bool
			if output, err := a.Command(ctx, "bmgr", "enabled").Output(); err != nil {
				s.Fatal("Failed to run adb command: ", err)
			} else {
				switch string(output) {
				case "Backup Manager currently enabled\n":
					enabled = true
				case "Backup Manager currently disabled\n":
					enabled = false
				default:
					s.Errorf("Invalid adb response: %q", string(output))
				}
			}

			if enabled != param.wantEnabled {
				s.Errorf("Unexpected ARC backup restore service state: got %t; want %t", enabled, param.wantEnabled)
			}
		})
	}
}
