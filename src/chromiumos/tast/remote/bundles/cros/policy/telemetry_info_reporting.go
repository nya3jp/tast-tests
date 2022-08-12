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

type telemetryInfoReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TelemetryInfoReporting,
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
				Name: "enabled",
				Val: telemetryInfoReportingParameters{
					usernamePath:     reportingutil.ReportingPoliciesEnabledUser,
					passwordPath:     reportingutil.ReportingPoliciesEnabledPassword,
					reportingEnabled: true,
				},
			}, {
				Name: "disabled",
				Val: telemetryInfoReportingParameters{
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

type TestType int

const (
	Info TestType = iota
	Telemetry
)

func TelemetryInfoReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(telemetryInfoReportingParameters)
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
		EnabledFeatures:    "EncryptedReportingPipeline, EnableTelemetryTestingRates",
		SkipLogin:          false,
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
		telemetryEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "TELEMETRY_METRIC")
		if err != nil {
			return errors.Wrap(err, "failed to look up telemetry events")
		}

		infoEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "INFO_METRIC")
		if err != nil {
			return errors.Wrap(err, "failed to look up info events")
		}
		for _, internalParam := range []struct {
			// name is the subtest name.
			name string
			// enum to know if telemetry or info
			test_type TestType
			// function to verify the event
			validator reportingutil.VerifyEventTypeCallback
		}{
			{
				name:      "audioTelemetry",
				test_type: Telemetry,
				validator: func(event reportingutil.InputEvent) bool {
					if w := event.WrappedEncryptedData; w != nil {
						if m := w.MetricData; m != nil {
							if i := m.TelemetryData; i != nil {
								if m := i.AudioTelemetry; m != nil {
									return true
								}
							}
						}
					}
					return false
				},
			},
			{
				name:      "networkTelemetry",
				test_type: Telemetry,
				validator: func(event reportingutil.InputEvent) bool {
					if w := event.WrappedEncryptedData; w != nil {
						if m := w.MetricData; m != nil {
							if i := m.TelemetryData; i != nil {
								if m := i.NetworkTelemetry; m != nil {
									return true
								}
							}
						}
					}
					return false
				},
			},
			{
				name:      "cpuInfo",
				test_type: Info,
				validator: func(event reportingutil.InputEvent) bool {
					if w := event.WrappedEncryptedData; w != nil {
						if m := w.MetricData; m != nil {
							if i := m.InfoData; i != nil {
								if m := i.CpuInfo; m != nil {
									return true
								}
							}
						}
					}
					return false
				},
			},
			{
				name:      "networkInfo",
				test_type: Info,
				validator: func(event reportingutil.InputEvent) bool {
					if w := event.WrappedEncryptedData; w != nil {
						if m := w.MetricData; m != nil {
							if i := m.InfoData; i != nil {
								if m := i.NetworkInfo; m != nil {
									return true
								}
							}
						}
					}
					return false
				},
			},
			{
				name:      "memoryInfo",
				test_type: Info,
				validator: func(event reportingutil.InputEvent) bool {
					if w := event.WrappedEncryptedData; w != nil {
						if m := w.MetricData; m != nil {
							if i := m.InfoData; i != nil {
								if m := i.MemoryInfo; m != nil {
									return true
								}
							}
						}
					}
					return false
				},
			},
		} {
			s.Run(ctx, internalParam.name, func(ctx context.Context, s *testing.State) {
				events := telemetryEvents
				if internalParam.test_type == Info {
					events = infoEvents
				}
				prunedEvents, err := reportingutil.PruneEvents(ctx, events, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
					return internalParam.validator(e)
				})
				if err != nil {
					testing.PollBreak(errors.Wrap(err, "failed to prune events"))
				}
				if !param.reportingEnabled && len(prunedEvents) == 0 {
					testing.ContextLog(ctx, "succeeded verifying test - reporting not enabled: ", internalParam.name)
				}
				if !param.reportingEnabled && len(prunedEvents) > 0 {
					s.Errorf("events found when reporting is disabled for %s", internalParam.name)
				}
				if param.reportingEnabled && internalParam.test_type == Telemetry && len(prunedEvents) > 2 {
					s.Errorf("more than one event reporting at test %s", internalParam.name)
				}
				if param.reportingEnabled && internalParam.test_type == Info && len(prunedEvents) > 1 {
					s.Errorf("more than one event reporting at test %s", internalParam.name)
				}
				if param.reportingEnabled && len(prunedEvents) == 0 {
					s.Errorf("no events found while reporting enabled at test %s", internalParam.name)
				}
			})
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  8 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate telemetry and info events: %v:", err)
	}
}
