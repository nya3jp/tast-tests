// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlputil

import (
	"context"
	"fmt"
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

// RestrictionBlockReportingEnabledUsername is the path to the secret username having block restriction level for all components and reporting enabled.
const RestrictionBlockReportingEnabledUsername = "dlp.restriction_level_block_reporting_enabled_username"

// RestrictionBlockReportingEnabledPassword is the path to the secret password having block restriction level for all components and reporting enabled.
const RestrictionBlockReportingEnabledPassword = "dlp.restriction_level_block_reporting_enabled_password"

// RestrictionBlockReportingDisabledUsername is the path to the secret username having block restriction level for all components and reporting disabled.
const RestrictionBlockReportingDisabledUsername = "dlp.restriction_level_block_reporting_disabled_username"

// RestrictionBlockReportingDisabledPassword is the path to the secret password having block restriction level for all components and reporting disabled.
const RestrictionBlockReportingDisabledPassword = "dlp.restriction_level_block_reporting_disabled_password"

// RestrictionReportReportingEnabledUsername is the path to the secret username having report restriction level for all components and reporting enabled.
const RestrictionReportReportingEnabledUsername = "dlp.restriction_level_report_reporting_enabled_username"

// RestrictionReportReportingEnabledPassword is the path to the secret password having report restriction level for all components and reporting enabled.
const RestrictionReportReportingEnabledPassword = "dlp.restriction_level_report_reporting_enabled_password"

// RestrictionReportReportingDisabledUsername is the path to the secret username having report restriction level for all components and reporting disabled.
const RestrictionReportReportingDisabledUsername = "dlp.restriction_level_report_reporting_disabled_username"

// RestrictionReportReportingDisabledPassword is the path to the secret password having report restriction level for all components and reporting disabled.
const RestrictionReportReportingDisabledPassword = "dlp.restriction_level_report_reporting_disabled_password"

// RestrictionWarnReportingEnabledUsername is the path to the secret username having warn restriction level for all components and reporting enabled.
const RestrictionWarnReportingEnabledUsername = "dlp.restriction_level_warn_reporting_enabled_username"

// RestrictionWarnReportingEnabledPassword is the path to the secret password having warn restriction level for all components and reporting enabled.
const RestrictionWarnReportingEnabledPassword = "dlp.restriction_level_warn_reporting_enabled_password"

// RestrictionWarnReportingDisabledUsername is the path to the secret username having warn restriction level for all components and reporting disabled.
const RestrictionWarnReportingDisabledUsername = "dlp.restriction_level_warn_reporting_disabled_username"

// RestrictionWarnReportingDisabledPassword is the path to the secret password having warn restriction level for all components and reporting disabled.
const RestrictionWarnReportingDisabledPassword = "dlp.restriction_level_warn_reporting_disabled_password"

// RestrictionAllowReportingEnabledUsername is the path to the secret username having allow restriction level for all components and reporting enabled.
const RestrictionAllowReportingEnabledUsername = "dlp.restriction_level_allow_reporting_enabled_username"

// RestrictionAllowReportingEnabledPassword is the path to the secret password having allow restriction level for all components and reporting enabled.
const RestrictionAllowReportingEnabledPassword = "dlp.restriction_level_allow_reporting_enabled_password"

// RestrictionAllowReportingDisabledUsername is the path to the secret username having allow restriction level for all components and reporting disabled.
const RestrictionAllowReportingDisabledUsername = "dlp.restriction_level_allow_reporting_disabled_username"

// RestrictionAllowReportingDisabledPassword is the path to the secret password having allow restriction level for all components and reporting disabled.
const RestrictionAllowReportingDisabledPassword = "dlp.restriction_level_allow_reporting_disabled_password"

// TestParams contains parameters for testing different DLP configurations.
type TestParams struct {
	Username         string          // username for Chrome enrollment
	Password         string          // password for Chrome enrollment
	Mode             dlp.Mode        // mode of the applied restriction
	BrowserType      dlp.BrowserType // which browser the test should use
	ReportingEnabled bool            // test should expect reporting to be enabled
}

// TestParameters contains the different configurations we want to test.
var TestParameters = []testing.Param{
	{
		Name: "ash_block_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionBlockReportingEnabledUsername,
			Password:         RestrictionBlockReportingEnabledPassword,
			Mode:             dlp.Mode_BLOCK,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: true,
		},
	},
	{
		Name: "ash_block_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionBlockReportingDisabledUsername,
			Password:         RestrictionBlockReportingDisabledPassword,
			Mode:             dlp.Mode_BLOCK,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: false,
		},
	},
	{
		Name: "lacros_block_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionBlockReportingEnabledUsername,
			Password:         RestrictionBlockReportingEnabledPassword,
			Mode:             dlp.Mode_BLOCK,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: true,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "lacros_block_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionBlockReportingDisabledUsername,
			Password:         RestrictionBlockReportingDisabledPassword,
			Mode:             dlp.Mode_BLOCK,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: false,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "ash_warn_cancel_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingEnabledUsername,
			Password:         RestrictionWarnReportingEnabledPassword,
			Mode:             dlp.Mode_WARN_CANCEL,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: true,
		},
	},
	{
		Name: "ash_warn_cancel_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingDisabledUsername,
			Password:         RestrictionWarnReportingDisabledPassword,
			Mode:             dlp.Mode_WARN_CANCEL,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: false,
		},
	},
	{
		Name: "lacros_warn_cancel_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingEnabledUsername,
			Password:         RestrictionWarnReportingEnabledPassword,
			Mode:             dlp.Mode_WARN_CANCEL,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: true,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "lacros_warn_cancel_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingDisabledUsername,
			Password:         RestrictionWarnReportingDisabledPassword,
			Mode:             dlp.Mode_WARN_CANCEL,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: false,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "ash_warn_proceed_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingEnabledUsername,
			Password:         RestrictionWarnReportingEnabledPassword,
			Mode:             dlp.Mode_WARN_PROCEED,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: true,
		},
	},
	{
		Name: "ash_warn_proceed_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingDisabledUsername,
			Password:         RestrictionWarnReportingDisabledPassword,
			Mode:             dlp.Mode_WARN_PROCEED,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: false,
		},
	},
	{
		Name: "lacros_warn_proceed_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingEnabledUsername,
			Password:         RestrictionWarnReportingEnabledPassword,
			Mode:             dlp.Mode_WARN_PROCEED,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: true,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "lacros_warn_proceed_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionWarnReportingDisabledUsername,
			Password:         RestrictionWarnReportingDisabledPassword,
			Mode:             dlp.Mode_WARN_PROCEED,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: false,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "ash_report_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionReportReportingEnabledUsername,
			Password:         RestrictionReportReportingEnabledPassword,
			Mode:             dlp.Mode_REPORT,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: true,
		},
	},
	{
		Name: "ash_report_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionReportReportingDisabledUsername,
			Password:         RestrictionReportReportingDisabledPassword,
			Mode:             dlp.Mode_REPORT,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: false,
		},
	},
	{
		Name: "lacros_report_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionReportReportingEnabledUsername,
			Password:         RestrictionReportReportingEnabledPassword,
			Mode:             dlp.Mode_REPORT,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: true,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "lacros_report_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionReportReportingDisabledUsername,
			Password:         RestrictionReportReportingDisabledPassword,
			Mode:             dlp.Mode_REPORT,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: false,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "ash_allow_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionAllowReportingEnabledUsername,
			Password:         RestrictionAllowReportingEnabledPassword,
			Mode:             dlp.Mode_ALLOW,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: true,
		},
	},
	{
		Name: "ash_allow_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionAllowReportingDisabledUsername,
			Password:         RestrictionAllowReportingDisabledPassword,
			Mode:             dlp.Mode_ALLOW,
			BrowserType:      dlp.BrowserType_ASH,
			ReportingEnabled: false,
		},
	},
	{
		Name: "lacros_allow_reporting_enabled",
		Val: TestParams{
			Username:         RestrictionAllowReportingEnabledUsername,
			Password:         RestrictionAllowReportingEnabledPassword,
			Mode:             dlp.Mode_ALLOW,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: true,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
	{
		Name: "lacros_allow_reporting_disabled",
		Val: TestParams{
			Username:         RestrictionAllowReportingDisabledUsername,
			Password:         RestrictionAllowReportingDisabledPassword,
			Mode:             dlp.Mode_ALLOW,
			BrowserType:      dlp.BrowserType_LACROS,
			ReportingEnabled: false,
		},
		ExtraSoftwareDeps: []string{"lacros"},
	},
}

// Action represents the supported DLP actions.
type Action int

const (
	// ClipboardCopyPaste identifies a clipboard copy and paste action.
	ClipboardCopyPaste Action = iota
	// Printing identifies a printing action.
	Printing
)

// String returns a string representation of `Action`.
func (action Action) String() string {
	switch action {
	case ClipboardCopyPaste:
		return "CLIPBOARD"
	case Printing:
		return "PRINTING"
	default:
		return fmt.Sprintf("String() not defined for Action %d", int(action))
	}
}

// retrieveEvents returns events for every restriction level having a timestamp greater than `testStartTime` with the given `clientID`.
func retrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time) ([]reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, error) {

	// Retrieve all DLP events stored in the server side.
	events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to look up events")
	}

	// Reduce events to those associate to `clientID` and starting after `testStartTime`.
	prunedEvents, err := reportingutil.PruneEvents(ctx, events, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return true
	})
	if err != nil {
		return nil, nil, nil, nil, errors.New("failed to prune events")
	}

	// Organize events according to their mode.
	var blockEvents []reportingutil.InputEvent
	var reportEvents []reportingutil.InputEvent
	var warnEvents []reportingutil.InputEvent
	var warnProceedEvents []reportingutil.InputEvent

	for _, event := range prunedEvents {
		mode := event.WrappedEncryptedData.DlpPolicyEvent.Mode
		switch {
		case mode == "BLOCK":
			blockEvents = append(blockEvents, event)
		case mode == "REPORT":
			reportEvents = append(reportEvents, event)
		case mode == "WARN":
			warnEvents = append(warnEvents, event)
		case mode == "WARN_PROCEED":
			warnProceedEvents = append(warnProceedEvents, event)
		default:
			return nil, nil, nil, nil, errors.New("unsupported event mode")
		}
	}

	return blockEvents, reportEvents, warnEvents, warnProceedEvents, nil

}

// validateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func validateEvents(reportingEnabled bool, mode dlp.Mode, action Action, blockEvents, reportEvents, warnEvents, warnProceedEvents []reportingutil.InputEvent) error {

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	var expectedEvents *[]reportingutil.InputEvent

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedEvents = &blockEvents
			expectedBlockEvents = 1
		case dlp.Mode_REPORT:
			expectedEvents = &reportEvents
			expectedReportEvents = 1
		case dlp.Mode_WARN_CANCEL:
			expectedEvents = &warnEvents
			expectedWarnEvents = 1
		case dlp.Mode_WARN_PROCEED:
			expectedEvents = &warnEvents
			expectedWarnEvents = 1
			expectedWarnProceedEvents = 1
		}
	}

	if len(blockEvents) != expectedBlockEvents ||
		len(reportEvents) != expectedReportEvents ||
		len(warnEvents) != expectedWarnEvents ||
		len(warnProceedEvents) != expectedWarnProceedEvents {
		return errors.Errorf("Expecting %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Got %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events instead",
			expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents,
			len(blockEvents), len(reportEvents), len(reportEvents), len(warnProceedEvents))
	}

	if (*expectedEvents)[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
		return errors.Errorf("Expecting %v restriction, got %v instead", Action.String(action), (*expectedEvents)[0].WrappedEncryptedData.DlpPolicyEvent.Restriction)
	}

	if mode == dlp.Mode_WARN_PROCEED {
		if warnProceedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("Expecting %v restriction, got %v instead", Action.String(action), warnProceedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction)
		}
	}

	return nil
}

// ValidateActionReporting tests the given action and checks whether the correct events are generated, sent, and received from the server side.
func ValidateActionReporting(ctx context.Context, s *testing.State, action Action) error {

	params := s.Param().(TestParams)

	username := s.RequiredVar(params.Username)
	password := s.RequiredVar(params.Password)
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
		EnableLacros:       params.BrowserType == dlp.BrowserType_LACROS,
		EnabledFeatures:    "EncryptedReportingPipeline",
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

	switch action {
	case ClipboardCopyPaste:
		service.ClipboardCopyPaste(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	case Printing:
		service.Print(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	}

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err = testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	blockEvents, reportEvents, warnEvents, warnProceedEvents, err := retrieveEvents(ctx, customerID, APIKey, c.ClientId, testStartTime)
	if err != nil {
		s.Fatal("Failed to retrieve events: ", err)
	}

	if err = validateEvents(params.ReportingEnabled, params.Mode, action, blockEvents, reportEvents, warnEvents, warnProceedEvents); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

	return nil

}
