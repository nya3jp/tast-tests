// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeartbeatReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify heartbeat reporting functionality",
		Contacts: []string{
			"tylergarrett@google.com", // Test owner
			"rzakarian@google.com",
			"cros-reporting-team@google.com",
		},
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      7 * time.Minute,
		VarDeps: []string{
			"policy.HeartbeatReporting.user_name",
			"policy.HeartbeatReporting.password",
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
	})
}

func HeartbeatReporting(ctx context.Context, s *testing.State) {
	user := s.RequiredVar("policy.HeartbeatReporting.user_name")
	pass := s.RequiredVar("policy.HeartbeatReporting.password")
	cID := s.RequiredVar(reportingutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(reportingutil.EventsAPIKeyPath)
	sa := []byte(s.RequiredVar(tape.ServiceAccountVar))

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	defer reportingutil.Deprovision(ctx, cl.Conn, sa, cID)

	policyClient := ps.NewPolicyServiceClient(cl.Conn)

	testStartTime := time.Now().Unix()
	if _, err := policyClient.GAIAEnrollForReporting(ctx, &ps.GAIAEnrollForReportingRequest{
		Username:           user,
		Password:           pass,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline, EncryptedReportingManualTestHeartbeatEvent",
		SkipLogin:          true,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := policyClient.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatalf("Failed to grab client ID from device: %v:", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, c.ClientId, APIKey, "HEARTBEAT_EVENTS", testStartTime)
		if err != nil {
			return errors.Wrap(err, "failed to look up events")
		}
		if len(events) < 1 {
			return errors.New("no event found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  2 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate heartbeat event: %v:", err)
	}
}
