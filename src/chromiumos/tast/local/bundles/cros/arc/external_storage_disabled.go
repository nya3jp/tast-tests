// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExternalStorageDisabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ExternalStorageDisabled policy is correctly applied to ARC",
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

func ExternalStorageDisabled(ctx context.Context, s *testing.State) {
	// Actual username and password are read from vars/arc.yaml.
	creds, err := chrome.PickRandomCreds(s.RequiredVar("arc.managedAccountPool"))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	policies := []policy.Policy{
		&policy.ArcEnabled{Val: true, Stat: policy.StatusSet},
		&policy.ExternalStorageDisabled{Val: true, Stat: policy.StatusSet},
	}

	fdms, err := policyutil.SetUpFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to set up fake policy server: ", err)
	}
	defer fdms.Stop(ctx)

	// If fdms forces ARC opt-in, then ARC opt-in will start in background, right after chrome is created.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(creds),
		chrome.DMSPolicy(fdms.URL),
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	s.Log("Performing MyFiles sharing check")

	if err := arc.WaitForARCMyFilesVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	const (
		imageSize = 64 * 1024 * 1024
		diskName  = "MyDisk"
	)

	// Create a filesystem image and mount it on the host side. This should work
	// even when the ExternalStorageDisabled policy is set to true since we
	// directly ask CrosDisks to mount it without having it check the policy
	// with Chrome here.
	_, cleanupFunc, err := removablemedia.CreateAndMountImage(ctx, imageSize, diskName)
	if err != nil {
		s.Fatal("Failed to set up image: ", err)
	}
	defer cleanupFunc(ctx)

	s.Log("Performing removable media sharing check")

	// Check that the image is not mounted on ARC.
	if err := arc.WaitForARCRemovableMediaVolumeMount(ctx, a); err == nil {
		s.Fatal("The volume is unexpectedly mounted on ARC")
	}
}
