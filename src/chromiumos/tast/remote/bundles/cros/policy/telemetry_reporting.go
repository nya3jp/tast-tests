// Copyright 2022 The ChromiumOS Authors.
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

const audioReportingEnabledUser = "policy.AudioReporting.enabled_username"
const audioReportingEnabledPassword = "policy.AudioReporting.password"

type telemetryReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TelemetryReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify memory reporting functionality",
		Contacts: []string{
			"albertojuarez@google.com", // Test owner
			"cros-reporting-team@google.com",
		},
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "telemetry_reporting_enabled",
				Val: telemetryReportingParameters{
					usernamePath:     audioReportingEnabledUser,
					passwordPath:     audioReportingEnabledPassword,
					reportingEnabled: true,
				},
			}, {
				Name: "telemetry_reporting_disabled",
				Val: telemetryReportingParameters{
					usernamePath:     reportingutil.ReportingPoliciesDisabledUser,
					passwordPath:     reportingutil.ReportingPoliciesDisabledPassword,
					reportingEnabled: false,
				},
			},
		},
		VarDeps: []string{
			//reportingutil.ReportingPoliciesEnabledUser,
			//reportingutil.ReportingPoliciesEnabledPassword,
			reportingutil.ReportingPoliciesDisabledUser,
			reportingutil.ReportingPoliciesDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
	})
}

func audioTelemetry(event reportingutil.InputEvent) *reportingutil.AudioTelemetry {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.MetricData; m != nil {
			if i := m.TelemetryData; i != nil {
				if m := i.AudioTelemetry; m != nil {
					return m
				}
			}
		}
	}
	return nil
}

func networkTelemetry(event reportingutil.InputEvent) *reportingutil.NetworkTelemetry {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.MetricData; m != nil {
			if i := m.TelemetryData; i != nil {
				if m := i.NetworkTelemetry; m != nil {
					return m
				}
			}
		}
	}
	return nil
}

func TelemetryReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(telemetryReportingParameters)
	user := s.RequiredVar(param.usernamePath)
	pass := s.RequiredVar(param.passwordPath)
	cID := s.RequiredVar(reportingutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(reportingutil.EventsAPIKeyPath)
	sa := []byte(s.RequiredVar(tape.ServiceAccountVar))

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
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
	defer reportingutil.Deprovision(ctx, cl.Conn, sa, cID)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	testStartTime := time.Now()
	if _, err := pc.GAIAEnrollForReporting(ctx, &ps.GAIAEnrollForReportingRequest{
		Username:           user,
		Password:           pass,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline , EnableTelemetryTestingRates",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}

	testing.ContextLog(ctx, "Waiting for 5 min to check for reported telemetry")
	if err = testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "HEARTBEAT_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up events")
		}

		prunedEventsAudio, err := reportingutil.PruneEvents(ctx, events, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return audioTelemetry(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune audio telemetry"))
		}

		prunedEventsNetwork, err := reportingutil.PruneEvents(ctx, events, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return networkTelemetry(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune network telemetry"))
		}

		if !param.reportingEnabled && len(prunedEventsAudio) == 0 && len(prunedEventsNetwork) == 0 {
			return nil
		}
		if !param.reportingEnabled && (len(prunedEventsAudio) > 0 || len(prunedEventsNetwork) > 0) {
			return testing.PollBreak(errors.New("events found when reporting is disabled"))
		}
		if param.reportingEnabled && (len(prunedEventsAudio) > 1 || len(prunedEventsNetwork) > 1) {
			return testing.PollBreak(errors.New("more than one event reporting"))
		}
		if param.reportingEnabled && (len(prunedEventsAudio) == 0 || len(prunedEventsNetwork) == 0) {
			return errors.New("no events found while reporting enabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  8 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate heartbeat event: %v:", err)
	}
}
