// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
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
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
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
					usernamePath:     reportingutil.ReportingPoliciesDisabledUser,
					passwordPath:     reportingutil.ReportingPoliciesDisabledPassword,
					reportingEnabled: false,
					vProSpecific:     false,
				},
			},
		},
		VarDeps: []string{
			memoryReportingEnabledUser,
			memoryReportingEnabledPassword,
			reportingutil.ReportingPoliciesDisabledUser,
			reportingutil.ReportingPoliciesDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
	})
}

// vProSupported checks if vPro features are on the device. Even on vPro supported models, vPro
// is not always supported on the specific device. Therefore some manual checking is required.
func vProSupported(ctx context.Context, conn *grpc.ClientConn) (bool, error) {
	fs := dutfs.NewClient(conn)
	out, err := fs.ReadFile(ctx, "/proc/cpuinfo")
	if err != nil {
		return false, errors.Wrap(err, "failed to read /proc/cpuinfo file")
	}
	// Checking whether the system supports vPro feature or not.
	if strings.Contains(string(out), "tme") {
		return true, nil
	}
	return false, nil
}

func tmeInfo(event reportingutil.InputEvent) *reportingutil.TMEInfo {
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

func validateTMEInfo(ctx context.Context, tmeInfo reportingutil.TMEInfo, vProSpecific bool) error {
	if vProSpecific {
		if keyLength, err := strconv.ParseInt(tmeInfo.KeyLength, 10, 64); err != nil {
			return errors.Wrap(err, "failed to parse key length")
		} else if keyLength <= 0 {
			return errors.Errorf("failed to verify key length: %v", keyLength)
		}
		if maxKeys, err := strconv.ParseInt(tmeInfo.MaxKeys, 10, 64); err != nil {
			return errors.Wrap(err, "failed to parse max keys")
		} else if maxKeys <= 0 {
			return errors.Errorf("failed to verify max keys: %v", maxKeys)
		}
		if tmeInfo.MemoryEncryptionState == "UNSPECIFIED_MEMORY_ENCRYPTION_STATE" {
			return errors.Errorf("failed to verify encryption state: %v", tmeInfo.MemoryEncryptionState)
		}
		if tmeInfo.MemoryEncryptionAlgorithm == "UNSPECIFIED_MEMORY_ENCRYPTION_ALGORITHM" {
			return errors.Errorf("failed to verify encryption algorithm: %v", tmeInfo.MemoryEncryptionAlgorithm)
		}
	} else {
		// For nonVpro cases, key length and max keys should be unset.
		if tmeInfo.KeyLength != "" {
			return errors.Errorf("failed to verify key length: %v", tmeInfo.KeyLength)
		}
		if tmeInfo.MaxKeys != "" {
			return errors.Errorf("failed to verify max keys: %v", tmeInfo.MaxKeys)
		}
		if tmeInfo.MemoryEncryptionState != "MEMORY_ENCRYPTION_STATE_DISABLED" {
			return errors.Errorf("failed to verify encryption state: %v", tmeInfo.MemoryEncryptionState)
		}
		if tmeInfo.MemoryEncryptionAlgorithm != "" {
			return errors.Errorf("failed to verify encryption algorithm: %v", tmeInfo.MemoryEncryptionAlgorithm)
		}
	}
	return nil
}

func MemoryReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(memoryReportingParameters)
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

	if param.vProSpecific {
		if su, err := vProSupported(ctx, cl.Conn); err != nil {
			s.Fatalf("Failed to verify if vPro is supported: %v: ", err)
		} else if !su {
			// vPro is not actually enabled on this device, skip test for now.
			testing.ContextLog(ctx, "not testing vPro reporting on non vpro device")
			return
		}
	}

	pc := ps.NewPolicyServiceClient(cl.Conn)

	testStartTime := time.Now().Unix()
	if _, err := pc.GAIAEnrollForReporting(ctx, &ps.GAIAEnrollForReportingRequest{
		Username:           user,
		Password:           pass,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline",
		SkipLogin:          true,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}

	// Events sent from the metric reporting manager won't be reported for the first minute.
	if err = testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, c.ClientId, APIKey, "INFO_METRIC", testStartTime)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to look up events"))
		}

		prunedEvents, err := reportingutil.PruneEvents(ctx, events, c.ClientId, func(e reportingutil.InputEvent) bool {
			return tmeInfo(e) != nil
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
		if err = validateTMEInfo(ctx, *tmeInfo(prunedEvents[0]), param.vProSpecific); err != nil {
			return testing.PollBreak(errors.Wrap(err, "invalid event"))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  3 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate heartbeat event: %v:", err)
	}
}
