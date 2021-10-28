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
		Func: ArcSnapshot,
		Desc: "Test taking ARC data/ snapshot",
		Contacts: []string{
			"pbond@chromium.org", // Test author
			"arc-commercial@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
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

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
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
	{
		s.Log("Running MGS without a snapshot")
		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "MgsWithoutSnapshot",
			IsHeadless: false,
			User:       "",
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Remote call WaitForPackagesInMgs() failed: ", err)
		}
		user = res.User
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}

	d := s.DUT()

	s.Log("Rebooting")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	{
		s.Log("Waiting for the first snapshot being taken")
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		service = enterprise.NewArcSnapshotServiceClient(cl.Conn)

		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "firstSnapshot",
			IsHeadless: true,
			User:       user,
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Failed to start taking a snapshot: ", err)
		}
		s.Log("Checking if the first snapshot is taken")
		_, err = service.WaitForSnapshot(ctx, &enterprise.WaitForSnapshotRequest{SnapshotNames: []string{"last"}})
		if err != nil {
			s.Fatal("Failed to take a snapshot: ", err)
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}

	{
		s.Log("Running MGS with the first snapshot")
		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "MgsWithFirstSnapshot",
			IsHeadless: false,
			User:       user,
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Failed to start MGS with a snapshot: ", err)
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}

	s.Log("Rebooting")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	{
		s.Log("Waiting for the second snapshot being taken")
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		service = enterprise.NewArcSnapshotServiceClient(cl.Conn)

		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "secondSnapshot",
			IsHeadless: true,
			User:       user,
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Failed to start taking a snapshot: ", err)
		}
		s.Log("Checking if the second snapshot is taken")
		_, err = service.WaitForSnapshot(ctx, &enterprise.WaitForSnapshotRequest{SnapshotNames: []string{"previous", "last"}})
		if err != nil {
			s.Fatal("Failed to take a snapshot: ", err)
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}

	{
		s.Log(ctx, "Running MGS with the second snapshot...")
		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "MgsWithSecondSnapshot",
			IsHeadless: false,
			User:       user,
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Failed to start MGS with a snapshot: ", err)
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}

	s.Log(ctx, "Rebooting...")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	{
		s.Log(ctx, "Running MGS with the second snapshot after reboot...")
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		service = enterprise.NewArcSnapshotServiceClient(cl.Conn)

		req := &enterprise.WaitForPackagesInMgsRequest{
			Name:       "MgsWithSnapshotAfterReboot",
			IsHeadless: false,
			User:       user,
			Packages:   packages,
		}
		res, err := service.WaitForPackagesInMgs(ctx, req)
		if err != nil {
			s.Fatal("Failed to start MGS with a second snapshot: ", err)
		}
		perfValues.Merge(perf.NewValuesFromProto(res.Perf))
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed to save perf results: ", err)
		}
	}
}
