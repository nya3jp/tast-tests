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

func validateAddedRemovedEvents(ctx context.Context, addedRemovedEvents []reportingutil.AddRemoveUserEvent, email string) error {
	addedEvent := addedRemovedEvents[0]
	removedEvent := addedRemovedEvents[1]
	if addedEvent.UserAddedEvent == nil {
		return errors.Errorf("didn't found the UserAddedEvent")
	}
	if addedEvent.AffiliatedUser.UserEmail != email {
		return errors.Errorf("AffiliatedUser email didn't matched on UserAddedEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, email)
	}
	if removedEvent.UserRemovedEvent == nil {
		return errors.Errorf("didn't found the UserAddedEvent")
	}
	if removedEvent.AffiliatedUser.UserEmail != email {
		return errors.Errorf("AffiliatedUser email didn't matched on UserAddedEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, email)
	}
	return nil
}

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
		//return nil, errors.Wrap(err, "failed to start chrome")
		s.Fatal("Failed to start chrome: ", err)
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
		s.Fatal("failed to create the keyboard: ", err)
	}
	testing.ContextLog(ctx, "Entering wrong password on login screen")
	if err := lockscreen.TypePassword(ctx, tconn, user, pass, kb); err != nil {
		s.Fatal("failed to type wrong password: ", err)
	}

	testing.ContextLog(ctx, "Entering correct password on login screen")
	if err := lockscreen.TypePassword(ctx, tconn, user, pass, kb); err != nil {
		s.Fatal("failed to type correct password: ", err)
	}

	if err := lockscreen.Unlock(ctx, tconn); err != nil {
		s.Fatal("Failed to unlock the screen: ", err)
	}

	if err := quicksettings.SignOut(ctx, tconn); err != nil {
		s.Fatal("failed to sign out with quick settings: ", err)
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
			return addRemoveUserEvent(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}
		if !param.reportingEnabled && len(prunedAddedRemovedEvents) > 0 {
			return errors.Errorf("added removed events found when reporting is disabled")
		}
		if param.reportingEnabled && len(prunedAddedRemovedEvents) > 2 {
			return errors.Errorf("more than two addRemoveUserEvent found")
		}
		if param.reportingEnabled && len(prunedAddedRemovedEvents) == 0 {
			return errors.Errorf("no addRemoveUserEvent found with reporting enabled")
		}
		addedEvent := *addRemoveUserEvent(addedRemovedEvents[0])
		removedEvent := *addRemoveUserEvent(addedRemovedEvents[1])
		if addedEvent.UserAddedEvent == nil {
			return errors.Errorf("didn't found the UserAddedEvent")
		}
		if addedEvent.AffiliatedUser.UserEmail != user {
			return errors.Errorf("AffiliatedUser email didn't matched on UserAddedEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, user)
		}
		if removedEvent.UserRemovedEvent == nil {
			return errors.Errorf("didn't found the UserAddedEvent")
		}
		if removedEvent.AffiliatedUser.UserEmail != user {
			return errors.Errorf("AffiliatedUser email didn't matched on UserAddedEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, user)
		}
		/*
			if err = validateAddedRemovedEvents(ctx, *addedRemovedEvents(prunedAddedRemovedEvents), user); err != nil {
				return testing.PollBreak(errors.Wrap(err, "invalid event"))
			}
		*/

		loginLogoutEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "LOGIN_LOGOUT_EVENTS")
		if err != nil {
			return errors.Wrap(err, "failed to look up info events")
		}
		prunedLoginLogoutEvents, err := reportingutil.PruneEvents(ctx, loginLogoutEvents, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return loginLogoutEvent(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}
		if !param.reportingEnabled && len(prunedLoginLogoutEvents) > 0 {
			return errors.Errorf("login logout events found when reporting is disabled")
		}
		if param.reportingEnabled && len(prunedLoginLogoutEvents) > 2 {
			return errors.Errorf("more than two loginLogoutEvent found")
		}
		if param.reportingEnabled && len(prunedLoginLogoutEvents) == 0 {
			return errors.Errorf("no loginLogoutEvent found with reporting enabled")
		}
		loginEvent := *loginLogoutEvent(prunedLoginLogoutEvents[0])
		logoutEvent := *loginLogoutEvent(prunedLoginLogoutEvents[1])
		if loginEvent.LoginEvent == nil {
			return errors.Errorf("didn't found the LoginEvent")
		}
		if loginEvent.AffiliatedUser.UserEmail != user {
			return errors.Errorf("AffiliatedUser email didn't matched on LoginLogoutEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, user)
		}
		if loginEvent.SessionType != "REGULAR_USER_SESSION" {
			return errors.Errorf("LoginEvent doesn't have the correct SessionType")
		}
		if logoutEvent.LogoutEvent == nil {
			return errors.Errorf("didn't found the LogoutEvent")
		}
		if logoutEvent.AffiliatedUser.UserEmail != user {
			return errors.Errorf("AffiliatedUser email didn't matched on LoginLogoutEvent, have %v wanted %v", addedEvent.AffiliatedUser.UserEmail, user)
		}
		if logoutEvent.SessionType != "REGULAR_USER_SESSION" {
			return errors.Errorf("LogoutEvent doesn't have the correct SessionType")
		}

		/*
			lockUnlockEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "LOCK_UNLOCK_EVENTS")
			if err != nil {
				return errors.Wrap(err, "failed to look up info events")
			}
			prunedLockUnlockEvents, err := reportingutil.PruneEvents(ctx, lockUnlockEvents, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
				return loginLogoutEvent(e) != nil
			})
			if err != nil {
				testing.PollBreak(errors.Wrap(err, "failed to prune events"))
			}
			if !param.reportingEnabled && len(prunedLockUnlockEvents) > 0 {
				return errors.Errorf("lock unlock events found when reporting is disabled")
			}
		*/

		/*
			if !param.reportingEnabled && len(prunedEvents) == 0 {
				testing.ContextLog(ctx, "succeeded verifying test - reporting disabled: ")
			}
			if !param.reportingEnabled && len(prunedEvents) > 0 {
				return errors.Errorf("events found when reporting is disabled")
			}

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
