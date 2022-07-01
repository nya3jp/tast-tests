// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const heartbeatEventDestination = "HEARTBEAT_EVENTS"

type testParameters struct {
	username             string // username for Chrome login
	password             string // password to login
	dmserver             string // device management server url
	reportingserver      string // reporting api server url
	obfuscatedcustomerid string // external customer id
	apikey               string // debug service key
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
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testParameters{
					username:             "policy.GAIAReporting.user_name",
					password:             "policy.GAIAReporting.password",
					dmserver:             "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					reportingserver:      "https://autopush-chromereporting-pa.sandbox.googleapis.com/v1",
					obfuscatedcustomerid: "policy.GAIAReporting.obfuscated_customer_id",
					apikey:               "policy.GAIAReporting.lookup_events_api_key",
				},
			},
		},
		Vars: []string{
			"policy.GAIAReporting.user_name",
			"policy.GAIAReporting.password",
			"policy.GAIAReporting.obfuscated_customer_id",
			"policy.GAIAReporting.lookup_events_api_key",
		},
	})
}

// validateEventReception Given a list of events received by the Reporting API Server, validate whether the list contains events sent by this test.
func validateEventReception(ctx context.Context, events []policyutil.InputEvent, clientID, eventDestination string, testStartTime time.Time) (bool, error) {
	for _, event := range events {
		if event.ClientID == clientID {
			// Parse the timestamp and check if the server received the event
			// after test started.
			micros, err := strconv.ParseInt(event.APIEvent.ReportingRecordEvent.Time, 10, 64)
			if err != nil {
				return false, errors.Wrap(err, "failed to parse int64 Spanner timestamp from event")
			}
			enqueueTime := time.Unix(micros/(int64(time.Second)/int64(time.Microsecond)), 0)
			if enqueueTime.After(testStartTime) {
				return true, nil
			}
		}
	}
	return false, nil
}

func HeartbeatReporting(ctx context.Context, s *testing.State) {
	testStartTime := time.Now()
	param := s.Param().(testParameters)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver
	reportingServerURL := param.reportingserver
	obfuscatedCustomerID := s.RequiredVar(param.obfuscatedcustomerid)
	APIKey := s.RequiredVar(param.apikey)

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
		Username:           username,
		Password:           password,
		DmserverUrl:        dmServerURL,
		ReportingServerUrl: reportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline, EncryptedReportingManualTestHeartbeatEvent",
		ExtraArgs:          "--login-manager",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatalf("Failed to grab client ID from device: %v:", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := policyutil.LookupEvents(ctx, reportingServerURL, obfuscatedCustomerID, APIKey, heartbeatEventDestination)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to look up events"))
		}

		if r, err := validateEventReception(ctx, events, c.ClientId, heartbeatEventDestination, testStartTime); err != nil {
			errors.Wrap(err, "error validating event")
		} else if !r {
			return errors.New("no event found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  240 * time.Second,
		Interval: 60 * time.Second,
	}); err != nil {
		errors.Wrap(err, "error when polling the reporting API")
		s.Errorf("Failed to validate heartbeat event: %v:", err)
	}
}
