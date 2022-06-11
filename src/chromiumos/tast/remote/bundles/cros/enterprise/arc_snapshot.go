// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/tape"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	enterprise "chromiumos/tast/services/cros/enterprise"
	ts "chromiumos/tast/services/cros/tape"
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
		ServiceDeps:  []string{"tast.cros.enterprise.ArcSnapshotService", "tast.cros.tape.Service"},
		SoftwareDeps: []string{"chrome", "reboot", "arc", "tpm2", "amd64"},
		Timeout:      40 * time.Minute,
		VarDeps: []string{
			"enterprise.ArcSnapshot.packages",
			"tape.service_account_key",
			"tape.managedchrome_id",
		},
	})
}

func ArcSnapshot(ctx context.Context, s *testing.State) {
	packages := strings.Split(s.RequiredVar("enterprise.ArcSnapshot.packages"), ",")

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	tapeClient, err := tape.NewTapeClient(ctx, tape.WithCredsJSON([]byte(s.RequiredVar("tape.service_account_key"))))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}

	poolID := "arc_snapshot"
	reqOTAparams := tape.RequestOTAParams{
		TimeoutInSeconds: 30,
		PoolID:           &poolID,
		Lock:             false,
	}
	acc, err := tape.RequestAccount(ctx, reqOTAparams, tapeClient)
	if err != nil {
		s.Fatal("RequestAccount failed: ", err)
	}

	defer func(ctx context.Context, tapeClient *http.Client) {
		err = acc.ReleaseAccount(ctx, tapeClient)
		if err != nil {
			s.Fatal("ReleaseAccount failed: ", err)
		}
	}(ctx, tapeClient)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := enterprise.NewArcSnapshotServiceClient(cl.Conn)

	s.Log("Enrolling device")
	if _, err = service.Enroll(ctx, &enterprise.EnrollRequest{User: acc.UserName, Pass: acc.Password}); err != nil {
		s.Fatal("Remote call Enroll() failed: ", err)
	}

	tapeService := ts.NewServiceClient(cl.Conn)
	customerID := s.RequiredVar("tape.managedchrome_id")
	// Get the device id of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: customerID})
	if err != nil {
		s.Fatal("Failed to get the deviceID: ", err)
	}

	// Deprovision the DUT at the end of the test.
	defer func(ctx context.Context) {
		var request tape.DeprovisionRequest
		request.DeviceID = res.DeviceID
		request.CustomerID = customerID

		if err = tape.Deprovision(ctx, request, tapeClient); err != nil {
			s.Fatalf("Failed to deprovision device %s: %v", request.DeviceID, err)
		}
	}(ctx)

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
