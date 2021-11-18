// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	enterprise "chromiumos/tast/services/cros/enterprise"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcSnapshot,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test taking ARC data/ snapshot",
		Contacts: []string{
			"pbond@chromium.org", // Test author
			"arc-commercial@google.com",
		},
		Attr:         []string{"group:arc-data-snapshot"},
		ServiceDeps:  []string{"tast.cros.enterprise.ArcSnapshotService"},
		SoftwareDeps: []string{"chrome", "reboot", "arc", "tpm2", "amd64"},
		Timeout:      40 * time.Minute,
		VarDeps: []string{
			"enterprise.ArcSnapshot.user",
			"enterprise.ArcSnapshot.pass",
			"enterprise.ArcSnapshot.packages",
		},
	})
}

func ArcSnapshot(ctx context.Context, s *testing.State) {
	enrollUser := s.RequiredVar("enterprise.ArcSnapshot.user")
	enrollPass := s.RequiredVar("enterprise.ArcSnapshot.pass")
	packages := strings.Split(s.RequiredVar("enterprise.ArcSnapshot.packages"), ",")

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := enterprise.NewArcSnapshotServiceClient(cl.Conn)

	s.Log("Enrolling device")
	if _, err = service.Enroll(ctx, &enterprise.EnrollRequest{User: enrollUser, Pass: enrollPass}); err != nil {
		s.Fatal("Remote call Enroll() failed: ", err)
	}

	user := ""
	perfValues := perf.NewValues()
	defer func(ctx context.Context) {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}(ctx)

	type RunParam struct {
		Request       *enterprise.WaitForPackagesInMgsRequest
		SnapshotNames []string
		NeedToReboot  bool
	}

	params := []RunParam{
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "MgsWithoutSnapshot",
				IsHeadless: false,
				Packages:   packages,
			},
			SnapshotNames: []string{},
			NeedToReboot:  true,
		},
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "firstSnapshot",
				IsHeadless: true,
				Packages:   packages,
			},
			SnapshotNames: []string{"last"},
			NeedToReboot:  false,
		},
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "MgsWithFirstSnapshot",
				IsHeadless: false,
				Packages:   packages,
			},
			SnapshotNames: []string{},
			NeedToReboot:  true,
		},
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "secondSnapshot",
				IsHeadless: true,
				Packages:   packages,
			},
			SnapshotNames: []string{"previous", "last"},
			NeedToReboot:  false,
		},
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "MgsWithSecondSnapshot",
				IsHeadless: false,
				Packages:   packages,
			},
			SnapshotNames: []string{},
			NeedToReboot:  true,
		},
		{
			Request: &enterprise.WaitForPackagesInMgsRequest{
				Name:       "MgsWithSecondSnapshotAfterReboot",
				IsHeadless: false,
				Packages:   packages,
			},
			SnapshotNames: []string{},
			NeedToReboot:  false,
		},
	}
	d := s.DUT()
	for _, param := range params {
		req := param.Request
		snapshotNames := param.SnapshotNames
		needToReboot := param.NeedToReboot

		s.Log("Running " + req.Name)
		req.User = user
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Remote call WaitForPackagesInMgs() failed: ", err)
		}
		user = res.User

		if len(snapshotNames) > 0 {
			s.Log("Checking if " + req.Name + " is taken")
			_, err = service.WaitForSnapshot(ctx, &enterprise.WaitForSnapshotRequest{SnapshotNames: snapshotNames})
			if err != nil {
				s.Fatal("Failed to take a snapshot: ", err)
			}
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))

		if needToReboot {
			s.Log("Rebooting")
			if err = d.Reboot(ctx); err != nil {
				s.Fatal("Failed to reboot DUT: ", err)
			}
			cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)
			service = enterprise.NewArcSnapshotServiceClient(cl.Conn)
		}
	}
}
