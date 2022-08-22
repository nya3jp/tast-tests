// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type telemetryInfoReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginLockReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device and verify memory reporting functionality",
		Contacts: []string{
			"albertojuarez@google.com", // Test owner
			"cros-reporting-team@google.com",
		},
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      15 * time.Minute,
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

func addRemoveUserEvent(event reportingutil.InputEvent) *reportingutil.AddRemoveUserEvent {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.AddRemoveUserEvent; m != nil {
			return m
		}
	}
	return nil
}

func loginLogoutEvent(event reportingutil.InputEvent) *reportingutil.LoginLogoutEvent {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.LoginLogoutEvent; m != nil {
			return m
		}
	}
	return nil
}

/*
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
*/
func LoginLockReporting(ctx context.Context, s *testing.State) {
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
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	defer reportingutil.Deprovision(ctx, cl.Conn, sa, cID)

	pc := pspb.NewPolicyServiceClient(cl.Conn)

	testStartTime := time.Now()

	cr, err := chrome.New(
		ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: user, Pass: pass}),
		chrome.GAIALogin(chrome.Creds{User: user, Pass: pass}),
		chrome.DMSPolicy(reportingutil.DmServerURL),
		chrome.EnableFeatures("EncryptedReportingPipeline"),
		chrome.EncryptedReportingAddr(fmt.Sprintf("%v/record", reportingutil.ReportingServerURL)),
		chrome.CustomLoginTimeout(chrome.EnrollmentAndLoginTimeout),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection")
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	testing.ContextLog(ctx, "Entering wrong password on login screen")
	if err := lockscreen.TypePassword(ctx, tconn, user, pass, kb); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	testing.ContextLog(ctx, "Entering correct password on login screen")
	if err := lockscreen.TypePassword(ctx, tconn, user, pass, kb); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := lockscreen.Unlock(ctx, tconn); err != nil {
		s.Fatal("Failed to unlock the screen: ", err)
	}

	if err := quicksettings.SignOut(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to sign out with quick settings")
	}

	/*
		have:
		1 added user
		1 login user
		1 lock
		1 unlock failed
		1 unlock succeeded
		1 logout
		missing:
		1 removed user
	*/

	// The info reporting normally takes a couple minutes to be reported but the
	// telemetry is reported every few hours if not using the
	// "EnableTelemetryTestingRates" feature enabled above which reports it
	// in 4-5 minutes.
	testing.ContextLog(ctx, "Waiting for 1 min to check for reported events")
	if err = testing.Sleep(ctx, 1*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		addedRemovedEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "ADDED_REMOVED_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up telemetry events")
		}
		prunedAddedRemovedEvents, err := reportingutil.PruneEvents(ctx, addedRemovedEvents, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return addedRemovedEvents(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}

		loginLogoutEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "LOGIN_LOGOUT_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up info events")
		}
		prunedLoginLogoutEvents, err := reportingutil.PruneEvents(ctx, addedRemovedEvents, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return loginLogoutEvent(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}

		lockUnlockEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "LOCK_UNLOCK_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up info events")
		}

		if !param.reportingEnabled && len(prunedEvents) == 0 {
			testing.ContextLog(ctx, "succeeded verifying test - reporting disabled: ")
		}
		if !param.reportingEnabled && len(prunedEvents) > 0 {
			return errors.Errorf("events found when reporting is disabled")
		}
		/*
			if param.reportingEnabled && internalParam.testType == Telemetry && len(prunedEvents) > 2 {
				return errors.Errorf("more than one event reporting at test %s with reportingEnabled set to %t", internalParam.name, param.reportingEnabled)
			}
			if param.reportingEnabled && internalParam.testType == Info && len(prunedEvents) > 1 {
				return errors.Errorf("more than one event reporting at test %s with reportingEnabled set to %t", internalParam.name, param.reportingEnabled)
			}
			if param.reportingEnabled && len(prunedEvents) == 0 {
				return errors.Errorf("no events found while reporting enabled at test %s with reportingEnabled set to %t", internalParam.name, param.reportingEnabled)
			}
			if param.reportingEnabled {
				testing.ContextLog(ctx, "succeeded verifying test - reporting enabled: ", internalParam.name)
			}
		*/
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Minute,
		Interval: 5 * time.Minute,
	}); err != nil {
		s.Errorf("Failed to validate telemetry and info events: %v:", err)
	}
}
