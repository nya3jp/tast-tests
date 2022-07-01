// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type heartbeatTestParams struct {
	username        string // username for Chrome login
	password        string // password to login
	dmserver        string // device management server url
	reportingserver string // reporting api server url
}

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
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
		VarDeps: []string{
			"policy.HeartbeatReporting.user_name",
			"policy.HeartbeatReporting.password",
			policyutil.ManagedChromeCustomerIDPath,
			policyutil.EventsAPIKeyPath,
		},
	})
}

// validateHeartbeatEvents Given a list of events received by the Reporting API Server, validate whether the list contains events sent by this test.
func validateHeartbeatEvents(ctx context.Context, events []policyutil.InputEvent, clientID string, testStartTime time.Time) (bool, error) {
	for _, event := range events {
		if event.ClientID == clientID {
			// Parse the timestamp and check if the server received the event
			// after test started.
			ms, err := strconv.ParseInt(event.APIEvent.ReportingRecordEvent.Time, 10, 64)
			if err != nil {
				return false, errors.Wrap(err, "failed to parse int64 Spanner timestamp from event")
			}
			enqueueTime := time.Unix(ms/(int64(time.Second)/int64(time.Microsecond)), 0)
			if enqueueTime.After(testStartTime) {
				j, _ := json.Marshal(event)
				testing.ContextLog(ctx, "Found a valid event: ", string(j))
				return true, nil
			}
		}
	}
	return false, nil
}

func HeartbeatReporting(ctx context.Context, s *testing.State) {
	testStartTime := time.Now()
	user := s.RequiredVar("policy.HeartbeatReporting.user_name")
	pass := s.RequiredVar("policy.HeartbeatReporting.password")
	cID := s.RequiredVar(policyutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(policyutil.EventsAPIKeyPath)

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.GAIAEnrollForReporting(ctx, &ps.GAIAEnrollForReportingRequest{
		Username:           user,
		Password:           pass,
		DmserverUrl:        policyutil.DmServerURL,
		ReportingServerUrl: policyutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline, EncryptedReportingManualTestHeartbeatEvent",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatalf("Failed to grab client ID from device: %v:", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := policyutil.LookupEvents(ctx, policyutil.ReportingServerUrl, cID, APIKey, "HEARTBEAT_EVENTS")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to look up events"))
		}

		if r, err := validateHeartbeatEvents(ctx, events, c.ClientId, testStartTime); err != nil {
			errors.Wrap(err, "error validating event")
		} else if !r {
			return errors.New("no event found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  2 * time.Minute,
		Interval: 1 * time.Minute,
	}); err != nil {
		errors.Wrap(err, "error when polling the reporting API")
		s.Errorf("Failed to validate heartbeat event: %v:", err)
	}
}
