// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"bytes"
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

type croshealthEventsReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CroshealthEventsReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify memory reporting functionality",
		Contacts: []string{
			"albertojuarez@google.com", // Test owner
			"cros-reporting-team@google.com",
		},
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      7 * time.Minute,
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{
			{
				Name: "reporting_enabled",
				Val: croshealthEventsReportingParameters{
					usernamePath:     reportingutil.ReportingPoliciesEnabledUser,
					passwordPath:     reportingutil.ReportingPoliciesEnabledPassword,
					reportingEnabled: true,
				},
			}, {
				Name: "reporting_disabled",
				Val: croshealthEventsReportingParameters{
					usernamePath:     reportingutil.ReportingPoliciesDisabledUser,
					passwordPath:     reportingutil.ReportingPoliciesDisabledPassword,
					reportingEnabled: false,
				},
			},
		},
		VarDeps: []string{
			reportingutil.ReportingPoliciesEnabledUser,
			reportingutil.ReportingPoliciesEnabledPassword,
			reportingutil.ReportingPoliciesDisabledUser,
			reportingutil.ReportingPoliciesDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
	})
}

func peripheralsTelemetry(event reportingutil.InputEvent) *reportingutil.PeripheralsTelemetry {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.MetricData; m != nil {
			if i := m.TelemetryData; i != nil {
				if m := i.PeripheralsTelemetry; m != nil {
					return m
				}
			}
		}
	}
	return nil
}

func CroshealthEventsReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(croshealthEventsReportingParameters)
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
	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=usb", "--length_seconds=10")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	// Trigger USB event.
	if err := testexec.CommandContext(ctx, "udevadm", "trigger", "-s", "usb", "-c", "add").Run(); err != nil {
		s.Fatal("Failed to trigger usb add event: ", err)
	}

	monitorCmd.Wait()

	stderr := string(stderrBuf.Bytes())
	if stderr != "" {
		s.Fatal("Failed to detect USB event, stderr: ", stderr)
	}

	stdout := string(stdoutBuf.Bytes())
	deviceAddedPattern := regexp.MustCompile(`"event": "Add"`)
	if !deviceAddedPattern.MatchString(stdout) {
		s.Fatal("Failed to detect USB event, event output: ", stdout)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "PERIPHERAL_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up events")
		}

		prunedEvents, err := reportingutil.PruneEvents(ctx, events, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return peripheralsTelemetry(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}
		if !param.reportingEnabled && len(prunedEvents) == 0 {
			return nil
		}
		if !param.reportingEnabled && len(prunedEvents) > 0 {
			return testing.PollBreak(errors.New("events found when reporting is disabled"))
		}
		if param.reportingEnabled && len(prunedEvents) > 1 {
			return testing.PollBreak(errors.New("more than one event reporting"))
		}
		if param.reportingEnabled && len(prunedEvents) == 0 {
			return errors.New("no events found while reporting enabled")
		}
		/*
			if param.reportingEnabled && len(prunedEvents[0].UsbTelemetry) == 0 {
				return errors.New("no events found while reporting enabled")
			}
			if param.reportingEnabled && len(prunedEvents[0].UsbTelemetry) > 1 {
				return testing.PollBreak(errors.New("more than one event reporting"))
			}
		*/

		return nil
	}, &testing.PollOptions{
		Timeout:  4 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate usb event: %v:", err)
	}
}
