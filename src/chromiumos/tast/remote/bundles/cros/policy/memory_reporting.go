// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const memoryReportingEnabledUser = "policy.MemoryReporting.enabled_username"
const memoryReportingEnabledPassword = "policy.MemoryReporting.password"

type memoryReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
	vProSpecific     bool   // test should prepare vPro specific logic
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify memory reporting functionality",
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
				Name:              "vpro_memory_reporting_enabled",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("brya", "redrix")),
				Val: memoryReportingParameters{
					usernamePath:     memoryReportingEnabledUser,
					passwordPath:     memoryReportingEnabledPassword,
					reportingEnabled: true,
					vProSpecific:     true,
				},
			}, {
				Name:              "nonvpro_memory_reporting_enabled",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("brya", "redrix")),
				Val: memoryReportingParameters{
					usernamePath:     memoryReportingEnabledUser,
					passwordPath:     memoryReportingEnabledPassword,
					reportingEnabled: true,
					vProSpecific:     false,
				},
			}, {
				Name: "memory_reporting_disabled",
				Val: memoryReportingParameters{
					usernamePath:     policyutil.ReportingPoliciesDisabledUser,
					passwordPath:     policyutil.ReportingPoliciesDisabledPassword,
					reportingEnabled: false,
					vProSpecific:     false,
				},
			},
		},
		VarDeps: []string{
			memoryReportingEnabledUser,
			memoryReportingEnabledPassword,
			policyutil.ReportingPoliciesDisabledUser,
			policyutil.ReportingPoliciesDisabledPassword,
			policyutil.ManagedChromeCustomerIDPath,
			policyutil.EventsAPIKeyPath,
		},
	})
}

// vProSupported checks if vPro features are on the device. Even on vPro supported models, vPro
// is not always supported on the specific device. Therefore some manual checking is required.
func vProSupported(ctx context.Context) (bool, error) {
	out, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false, errors.Wrap(err, "failed to read /proc/cpuinfo file")
	}
	// Checking whether the system supports vPro feature or not.
	if strings.Contains(string(out), "tme") {
		return true, nil
	}
	return false, nil
}

func tmeInfo(event policyutil.InputEvent) *policyutil.TMEInfo {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.MetricData; m != nil {
			if i := m.InfoData; i != nil {
				if m := i.MemoryInfo; m != nil {
					if t := m.TMEInfo; t != nil {
						return t
					}
				}
			}
		}
	}
	return nil
}

// pruneEvents Reduces the events response to only memory events after test began.
func pruneEvents(ctx context.Context, events []policyutil.InputEvent, clientID string, testStartTime time.Time) ([]policyutil.InputEvent, error) {
	var prunedEvents []policyutil.InputEvent
	for _, event := range events {
		if event.ClientID != clientID {
			continue
		}
		if tmeInfo(event) == nil {
			continue
		}
		ms, err := strconv.ParseInt(event.APIEvent.ReportingRecordEvent.Time, 10, 64)
		if err != nil {
			return prunedEvents, errors.Wrap(err, "failed to parse int64 Spanner timestamp from event")
		}
		enqueueTime := time.Unix(ms/(int64(time.Second)/int64(time.Microsecond)), 0)
		if enqueueTime.After(testStartTime) {
			prunedEvents = append(prunedEvents, event)
			j, _ := json.Marshal(event)
			testing.ContextLog(ctx, "Found a valid event ", string(j))
		}
	}

	return prunedEvents, nil
}

func MemoryReporting(ctx context.Context, s *testing.State) {
	testStartTime := time.Now()
	param := s.Param().(memoryReportingParameters)
	user := s.RequiredVar(param.usernamePath)
	pass := s.RequiredVar(param.passwordPath)
	cID := s.RequiredVar(policyutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(policyutil.EventsAPIKeyPath)

	supported, err := vProSupported(ctx)
	if err != nil {
		s.Fatalf("Failed to verify if vPro is supported: %v: ", err)
	}
	if (param.vProSpecific && !supported) || (!param.vProSpecific && supported) {
		testing.ContextLog(ctx, "not testing vPro reporting on non vpro device")
		return
	}

	if param.vProSpecific {
		if su, err := vProSupported(ctx); err != nil {
			s.Fatalf("Failed to verify if vPro is supported: %v: ", err)
		} else if !su {
			// vPro is not actually enabled on this device, skip test for now.
			return
		}
	}

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
		EnabledFeatures:    "EncryptedReportingPipeline",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}

	// Give time for reporting to reach the server. Note events sent from the metric reporting manager
	// won't be reported for the first minute.
	testing.Sleep(90 * time.Second)

	events, err := policyutil.LookupEvents(ctx, policyutil.ReportingServerURL, cID, APIKey, "INFO_METRIC")
	if err != nil {
		s.Fatal("Failed to look up events: ", err)
	}

	prunedEvents, err := pruneEvents(ctx, events, c.ClientId, testStartTime)
	if err != nil {
		s.Fatal("Failed to prune events: ", err)
	}

	if !param.reportingEnabled {
		if len(prunedEvents) > 0 {
			s.Error("Events founds when reporting is disabled: ", prunedEvents)
		}
		return
	}

	if len(prunedEvents) > 1 {
		s.Error("More than one event found: ", prunedEvents)
		return
	} else if len(prunedEvents) == 0 {
		s.Error("No events found")
		return
	}

	tmeInfo := tmeInfo(prunedEvents[0])
	if param.vProSpecific {
		if tmeInfo.KeyLength <= 0 {
			s.Error("Failed to verify key length: ", tmeInfo.KeyLength)
		}
		if tmeInfo.MaxKeys <= 0 {
			s.Error("Failed to verify max keys: ", tmeInfo.MaxKeys)
		}
		if tmeInfo.MemoryEncryptionState == "UNSPECIFIED_MEMORY_ENCRYPTION_STATE" {
			s.Error("Failed to verify encryption state: ", tmeInfo.MemoryEncryptionState)
		}
		if tmeInfo.MemoryEncryptionAlgorithm == "UNSPECIFIED_MEMORY_ENCRYPTION_ALGORITHM" {
			s.Error("Failed to verify encryption algorithm: ", tmeInfo.MemoryEncryptionAlgorithm)
		}
	} else {
		if tmeInfo.KeyLength != 0 {
			s.Error("Failed to verify key length: ", tmeInfo.KeyLength)
		}
		if tmeInfo.MaxKeys != 0 {
			s.Error("Failed to verify max keys: ", tmeInfo.MaxKeys)
		}
		if tmeInfo.MemoryEncryptionState != "MEMORY_ENCRYPTION_STATE_DISABLED" {
			s.Error("Failed to verify encryption state: ", tmeInfo.MemoryEncryptionState)
		}
		if tmeInfo.MemoryEncryptionAlgorithm != "" {
			s.Error("Failed to verify encryption algorithm: ", tmeInfo.MemoryEncryptionAlgorithm)
		}
	}
}
