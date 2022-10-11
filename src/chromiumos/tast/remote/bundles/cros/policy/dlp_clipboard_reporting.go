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
	dlp "chromiumos/tast/services/cros/dlp"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const restrictionBlockReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_block_reporting_enabled_username"
const restrictionBlockReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_block_reporting_enabled_password"

const restrictionBlockReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_block_reporting_disabled_username"
const restrictionBlockReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_block_reporting_disabled_password"

const restrictionReportReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_report_reporting_enabled_username"
const restrictionReportReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_report_reporting_enabled_password"

const restrictionReportReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_report_reporting_disabled_username"
const restrictionReportReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_report_reporting_disabled_password"

const restrictionWarnReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_warn_reporting_enabled_username"
const restrictionWarnReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_warn_reporting_enabled_password"

const restrictionWarnReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_warn_reporting_disabled_username"
const restrictionWarnReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_warn_reporting_disabled_password"

const restrictionAllowReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_allow_reporting_enabled_username"
const restrictionAllowReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_allow_reporting_enabled_password"

const restrictionAllowReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_allow_reporting_disabled_username"
const restrictionAllowReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_allow_reporting_disabled_password"

// A struct containing parameters for different clipboard tests.
type clipboardTestParams struct {
	username         string
	password         string
	mode             dlp.Mode
	browserType      dlp.BrowserType
	reportingEnabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpClipboardReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior whether clipboard copy and paste events are correctly reported when the restriction level is block, warn or report",
		Contacts: []string{
			"accorsi@google.com", // Test author
			"chromeos-dlp@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.dlp.DataLeakPreventionService",
			"tast.cros.browser.ChromeService",
			"tast.cros.policy.PolicyService",
			"tast.cros.tape.Service",
		},
		Timeout: 7 * time.Minute,
		VarDeps: []string{
			restrictionBlockReportingEnabledUsername,
			restrictionBlockReportingEnabledPassword,
			restrictionBlockReportingDisabledUsername,
			restrictionBlockReportingDisabledPassword,
			restrictionWarnReportingEnabledUsername,
			restrictionWarnReportingEnabledPassword,
			restrictionWarnReportingDisabledUsername,
			restrictionWarnReportingDisabledPassword,
			restrictionReportReportingEnabledUsername,
			restrictionReportReportingEnabledPassword,
			restrictionReportReportingDisabledUsername,
			restrictionReportReportingDisabledPassword,
			restrictionAllowReportingEnabledUsername,
			restrictionAllowReportingEnabledPassword,
			restrictionAllowReportingDisabledUsername,
			restrictionAllowReportingDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
		Params: []testing.Param{
			{
				Name: "test",
				Val: clipboardTestParams{
					username:         restrictionBlockReportingEnabledUsername,
					password:         restrictionBlockReportingEnabledPassword,
					mode:             dlp.Mode_Block,
					browserType:      dlp.BrowserType_Lacros,
					reportingEnabled: true,
				},
			},
			// {
			// 	Name: "ash_block_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionBlockReportingEnabledUsername,
			// 		password:         restrictionBlockReportingEnabledUsername,
			// 		mode:             dlp.Mode_Block,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "ash_block_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionBlockReportingDisabledUsername,
			// 		password:         restrictionBlockReportingDisabledPassword,
			// 		mode:             dlp.Mode_Block,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "lacros_block_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionBlockReportingEnabledUsername,
			// 		password:         restrictionBlockReportingEnabledUsername,
			// 		mode:             dlp.Mode_Block,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "lacros_block_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionBlockReportingDisabledUsername,
			// 		password:         restrictionBlockReportingDisabledPassword,
			// 		mode:             dlp.Mode_Block,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "ash_warn_cancel_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingEnabledUsername,
			// 		password:         restrictionWarnReportingEnabledUsername,
			// 		mode:             dlp.Mode_WarnCancel,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "ash_warn_cancel_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingDisabledUsername,
			// 		password:         restrictionWarnReportingDisabledPassword,
			// 		mode:             dlp.Mode_WarnCancel,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "lacros_warn_cancel_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingEnabledUsername,
			// 		password:         restrictionWarnReportingEnabledUsername,
			// 		mode:             dlp.Mode_WarnCancel,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "lacros_warn_cancel_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingDisabledUsername,
			// 		password:         restrictionWarnReportingDisabledPassword,
			// 		mode:             dlp.Mode_WarnCancel,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "ash_warn_proceed_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingEnabledUsername,
			// 		password:         restrictionWarnReportingEnabledUsername,
			// 		mode:             dlp.Mode_WarnProceed,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "ash_warn_proceed_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingDisabledUsername,
			// 		password:         restrictionWarnReportingDisabledPassword,
			// 		mode:             dlp.Mode_WarnProceed,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "lacros_warn_proceed_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingEnabledUsername,
			// 		password:         restrictionWarnReportingEnabledUsername,
			// 		mode:             dlp.Mode_WarnProceed,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "lacros_warn_proceed_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionWarnReportingDisabledUsername,
			// 		password:         restrictionWarnReportingDisabledPassword,
			// 		mode:             dlp.Mode_WarnProceed,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "ash_report_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionReportReportingEnabledUsername,
			// 		password:         restrictionReportReportingEnabledUsername,
			// 		mode:             dlp.Mode_Report,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "ash_report_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionReportReportingDisabledUsername,
			// 		password:         restrictionReportReportingDisabledPassword,
			// 		mode:             dlp.Mode_Report,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "lacros_report_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionReportReportingEnabledUsername,
			// 		password:         restrictionReportReportingEnabledUsername,
			// 		mode:             dlp.Mode_Report,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "lacros_report_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionReportReportingDisabledUsername,
			// 		password:         restrictionReportReportingDisabledPassword,
			// 		mode:             dlp.Mode_Report,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "ash_allow_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionAllowReportingEnabledUsername,
			// 		password:         restrictionAllowReportingEnabledUsername,
			// 		mode:             dlp.Mode_Allow,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "ash_allow_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionAllowReportingDisabledUsername,
			// 		password:         restrictionAllowReportingDisabledPassword,
			// 		mode:             dlp.Mode_Allow,
			// 		browserType:      dlp.BrowserType_Ash,
			// 		reportingEnabled: false,
			// 	},
			// },
			// {
			// 	Name: "lacros_allow_reporting_enabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionAllowReportingEnabledUsername,
			// 		password:         restrictionAllowReportingEnabledUsername,
			// 		mode:             dlp.Mode_Allow,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: true,
			// 	},
			// },
			// {
			// 	Name: "lacros_allow_reporting_disabled",
			// 	Val: clipboardTestParams{
			// 		username:         restrictionAllowReportingDisabledUsername,
			// 		password:         restrictionAllowReportingDisabledPassword,
			// 		mode:             dlp.Mode_Allow,
			// 		browserType:      dlp.BrowserType_Lacros,
			// 		reportingEnabled: false,
			// 	},
			// },
		},
	})
}

// dlpPolicyEventClipboard identifies clipboard events.
func dlpPolicyEventClipboard(event reportingutil.InputEvent, modeText string) bool {
	if w := event.WrappedEncryptedData; w != nil {
		if d := w.DlpPolicyEvent; d != nil && d.Restriction == "CLIPBOARD" && (d.Mode == modeText || len(d.Mode) == 0) {
			return true
		}
	}
	return false
}

// retrieveEvents returns events having a timestamp greater than `testStartTime` with the given `clientID` and satisfying `correctEventType`.
func retrieveEvents(ctx context.Context, s *testing.State, customerID, APIKey, clientID string, testStartTime time.Time, correctEventType func(reportingutil.InputEvent, string) bool, modeText string) ([]reportingutil.InputEvent, error) {

	events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	// Fatal error occurred while looking up events.
	if err != nil {
		return nil, errors.Wrap(err, "failed to look up events")
	}

	prunedEvents, err := reportingutil.PruneEvents(ctx, events, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return dlpPolicyEventClipboard(e, modeText)
	})
	if err != nil {
		return nil, errors.New("failed to prune events")
	}

	return prunedEvents, nil

}

func DlpClipboardReporting(ctx context.Context, s *testing.State) {
	params := s.Param().(clipboardTestParams)

	username := s.RequiredVar(params.username)
	password := s.RequiredVar(params.password)
	customerID := s.RequiredVar(reportingutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(reportingutil.EventsAPIKeyPath)
	sa := []byte(s.RequiredVar(tape.ServiceAccountVar))

	// Reset the DUT state.
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	// Establish RPC connection to the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	defer reportingutil.Deprovision(ctx, cl.Conn, sa, customerID)

	// Create client instance of the DataLeakPrevention service.
	service := dlp.NewDataLeakPreventionServiceClient(cl.Conn)

	// Use the service to enroll the DUT and login.
	if _, err = service.EnrollAndLogin(ctx, &dlp.EnrollAndLoginRequest{
		Username:           username,
		Password:           password,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline, LacrosSupport",
	}); err != nil {
		s.Fatal("Remote call EnrollAndLogin() failed: ", err)
	}

	// Create client instance of the Policy service just to retrieve the clientID.
	pc := ps.NewPolicyServiceClient(cl.Conn)

	// TODO(accorsi): consider whether porting this method to the DataLeakPrevention service.
	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}

	// We are going to filter the events also based on the test time.
	testStartTime := time.Now()

	// Perform a copy and paste action.
	service.ClipboardCopyPaste(ctx, &dlp.ClipboardCopyPasteRequest{
		BrowserType: params.browserType,
		Mode:        params.mode,
	})

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err = testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	expectedNumRetrievedEvents := 1
	if !params.reportingEnabled || params.mode == dlp.Mode_Allow {
		expectedNumRetrievedEvents = 0
	}

	blockEvents, err := retrieveEvents(ctx, s, customerID, APIKey, c.ClientId, testStartTime, dlpPolicyEventClipboard, "BLOCK")
	if err != nil {
		s.Fatal("Error while retrieving BLOCK events: ", err)
	}
	reportEvents, err := retrieveEvents(ctx, s, customerID, APIKey, c.ClientId, testStartTime, dlpPolicyEventClipboard, "REPORT")
	if err != nil {
		s.Fatal("Error while retrieving REPORT events: ", err)
	}
	warnEvents, err := retrieveEvents(ctx, s, customerID, APIKey, c.ClientId, testStartTime, dlpPolicyEventClipboard, "WARN")
	if err != nil {
		s.Fatal("Error while retrieving WARN events: ", err)
	}
	warnProceedEvents, err := retrieveEvents(ctx, s, customerID, APIKey, c.ClientId, testStartTime, dlpPolicyEventClipboard, "WARN_PROCEED")
	if err != nil {
		s.Fatal("Error while retrieving WARN_PROCEED events: ", err)
	}

	switch params.mode {
	case dlp.Mode_Block:
		if len(blockEvents) != expectedNumRetrievedEvents {
			s.Fatalf("Expecting %d BLOCK events, got %d instead", expectedNumRetrievedEvents, len(blockEvents))
		}
		if len(reportEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			s.Fatalf("Expecting 0 REPORT, 0 WARN, and 0 WARN_PROCEED events. Got %d REPORT, %d WARN, and % WARN_PROCEED events instead",
				len(reportEvents), len(reportEvents), len(warnProceedEvents))
		}
	case dlp.Mode_Report:
		if len(reportEvents) != expectedNumRetrievedEvents {
			s.Fatalf("Expecting %d REPORT events, got %d instead", expectedNumRetrievedEvents, len(reportEvents))
		}
		if len(blockEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			s.Fatalf("Expecting 0 BLOCK, 0 WARN, and 0 WARN_PROCEED events. Got %d BLOCK, %d WARN, and %d WARN_PROCEED events instead",
				len(blockEvents), len(warnEvents), len(warnProceedEvents))
		}
	case dlp.Mode_WarnCancel:
		if len(warnEvents) != expectedNumRetrievedEvents {
			s.Fatalf("Expecting %d WARN events, got %d instead", expectedNumRetrievedEvents, len(warnEvents))
		}
		if len(blockEvents) != 0 || len(reportEvents) != 0 || len(warnProceedEvents) != 0 {
			s.Fatalf("Expecting 0 BLOCK, 0 REPORT, and 0 WARN_PROCEED events. Got %d BLOCK, %d REPORT, and %d WARN_PROCEED events instead",
				len(blockEvents), len(reportEvents), len(warnProceedEvents))
		}
	case dlp.Mode_WarnProceed:
		if len(warnEvents) != expectedNumRetrievedEvents || len(warnProceedEvents) != expectedNumRetrievedEvents {
			s.Fatalf("Expecting %d WARN and %d WARN_PROCEED events. Got %d and %d instead",
				expectedNumRetrievedEvents, expectedNumRetrievedEvents, len(warnEvents), len(warnProceedEvents))
		}
		if len(blockEvents) != 0 || len(reportEvents) != 0 {
			s.Fatalf("Expecting 0 BLOCK and 0 REPORT events. Got %d BLOCK and %d REPORT events instead",
				len(blockEvents), len(reportEvents))
		}
	case dlp.Mode_Allow:
		if len(reportEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			s.Fatalf("Expecting 0 BLOCK, 0 REPORT, 0 WARN, and 0 WARN_PROCEED events. Got %d BLOCK, %d REPORT, %d WARN, and % WARN_PROCEED events instead",
				len(blockEvents), len(reportEvents), len(reportEvents), len(warnProceedEvents))
		}
	}

}
