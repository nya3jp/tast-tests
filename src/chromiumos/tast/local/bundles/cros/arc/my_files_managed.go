// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MyFilesManaged,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that MyFiles sharing works for managed users",
		Contacts: []string{
			"arc-storage@google.com",
			"momohatt@google.com",
		},
		Attr:         []string{"group:mainline", "group:arc-functional"},
		VarDeps:      []string{"arc.managedAccountPool"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

func MyFilesManaged(ctx context.Context, s *testing.State) {
	// Actual username and password are read from vars/arc.yaml.
	creds, err := chrome.PickRandomCreds(s.RequiredVar("arc.managedAccountPool"))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	policies := []policy.Policy{
		&policy.ArcEnabled{Val: true, Stat: policy.StatusSet},
		&policy.ExternalStorageDisabled{Val: true, Stat: policy.StatusSet},
	}

	fdms, err := arc.SetupFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(ctx)

	gaiaLogin := chrome.GAIALogin(creds)
	cr, err := arc.SetupManagedChrome(ctx, gaiaLogin, fdms)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer cr.Close(ctx)

	// ARC setup
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	a, err := arc.NewWithSyslogReader(ctx, s.OutDir(), reader)
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	s.Log("Performing MyFiles sharing check")

	if err := arc.WaitForARCMyFilesVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}
}
