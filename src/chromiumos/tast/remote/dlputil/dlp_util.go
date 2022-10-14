// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlputil

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	configpb "go.chromium.org/chromiumos/config/go/api"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	frameworkprotocol "chromiumos/tast/framework/protocol"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	dlp "chromiumos/tast/services/cros/dlp"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// RestrictionBlockReportingEnabledUsername is the path to the secret username having block restriction level for all components and reporting enabled.
	RestrictionBlockReportingEnabledUsername = "dlp.restriction_level_block_reporting_enabled_username"

	// RestrictionBlockReportingEnabledPassword is the path to the secret password having block restriction level for all components and reporting enabled.
	RestrictionBlockReportingEnabledPassword = "dlp.restriction_level_block_reporting_enabled_password"

	// RestrictionBlockReportingDisabledUsername is the path to the secret username having block restriction level for all components and reporting disabled.
	RestrictionBlockReportingDisabledUsername = "dlp.restriction_level_block_reporting_disabled_username"

	// RestrictionBlockReportingDisabledPassword is the path to the secret password having block restriction level for all components and reporting disabled.
	RestrictionBlockReportingDisabledPassword = "dlp.restriction_level_block_reporting_disabled_password"

	// RestrictionReportReportingEnabledUsername is the path to the secret username having report restriction level for all components and reporting enabled.
	RestrictionReportReportingEnabledUsername = "dlp.restriction_level_report_reporting_enabled_username"

	// RestrictionReportReportingEnabledPassword is the path to the secret password having report restriction level for all components and reporting enabled.
	RestrictionReportReportingEnabledPassword = "dlp.restriction_level_report_reporting_enabled_password"

	// RestrictionReportReportingDisabledUsername is the path to the secret username having report restriction level for all components and reporting disabled.
	RestrictionReportReportingDisabledUsername = "dlp.restriction_level_report_reporting_disabled_username"

	// RestrictionReportReportingDisabledPassword is the path to the secret password having report restriction level for all components and reporting disabled.
	RestrictionReportReportingDisabledPassword = "dlp.restriction_level_report_reporting_disabled_password"

	// RestrictionWarnReportingEnabledUsername is the path to the secret username having warn restriction level for all components and reporting enabled.
	RestrictionWarnReportingEnabledUsername = "dlp.restriction_level_warn_reporting_enabled_username"

	// RestrictionWarnReportingEnabledPassword is the path to the secret password having warn restriction level for all components and reporting enabled.
	RestrictionWarnReportingEnabledPassword = "dlp.restriction_level_warn_reporting_enabled_password"

	// RestrictionWarnReportingDisabledUsername is the path to the secret username having warn restriction level for all components and reporting disabled.
	RestrictionWarnReportingDisabledUsername = "dlp.restriction_level_warn_reporting_disabled_username"

	// RestrictionWarnReportingDisabledPassword is the path to the secret password having warn restriction level for all components and reporting disabled.
	RestrictionWarnReportingDisabledPassword = "dlp.restriction_level_warn_reporting_disabled_password"

	// RestrictionAllowReportingEnabledUsername is the path to the secret username having allow restriction level for all components and reporting enabled.
	RestrictionAllowReportingEnabledUsername = "dlp.restriction_level_allow_reporting_enabled_username"

	// RestrictionAllowReportingEnabledPassword is the path to the secret password having allow restriction level for all components and reporting enabled.
	RestrictionAllowReportingEnabledPassword = "dlp.restriction_level_allow_reporting_enabled_password"

	// RestrictionAllowReportingDisabledUsername is the path to the secret username having allow restriction level for all components and reporting disabled.
	RestrictionAllowReportingDisabledUsername = "dlp.restriction_level_allow_reporting_disabled_username"

	// RestrictionAllowReportingDisabledPassword is the path to the secret password having allow restriction level for all components and reporting disabled.
	RestrictionAllowReportingDisabledPassword = "dlp.restriction_level_allow_reporting_disabled_password"
)

// TestParams contains parameters for testing different DLP configurations.
type TestParams struct {
	Username         string          // username for Chrome enrollment
	Password         string          // password for Chrome enrollment
	Mode             dlp.Mode        // mode of the applied restriction
	BrowserType      dlp.BrowserType // which browser the test should use
	ReportingEnabled bool            // test should expect reporting to be enabled
}

// Action represents the supported DLP actions.
type Action int

const (
	// ClipboardCopyPaste identifies a clipboard copy and paste action.
	ClipboardCopyPaste Action = iota
	// Printing identifies a printing action.
	Printing
	// Screenshot identifies a screenshot action.
	Screenshot
	// PrivacyScreen identifies a privacy screen enforcement.
	PrivacyScreen
)

// String returns a string representation of `Action`.
func (action Action) String() string {
	switch action {
	case ClipboardCopyPaste:
		return "CLIPBOARD"
	case Printing:
		return "PRINTING"
	case Screenshot:
		return "SCREENSHOT"
	case PrivacyScreen:
		return "EPRIVACY"
	default:
		return fmt.Sprintf("String() not defined for Action %d", int(action))
	}
}

// eventsBundle contains an events vector per restriction level and it is populated by `retrieveEvents`.
type eventsBundle struct {
	block       []reportingutil.InputEvent
	report      []reportingutil.InputEvent
	warn        []reportingutil.InputEvent
	warnProceed []reportingutil.InputEvent
}

// retrieveEvents returns events for every restriction level having a timestamp greater than `testStartTime` with the given `clientID`.
func retrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time) (*eventsBundle, error) {

	// Retrieve all DLP events stored in the server side.
	dlpEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	if err != nil {
		return nil, errors.Wrap(err, "failed to look up events")
	}

	// Reduce events to those associate to `clientID` and starting after `testStartTime`.
	prunedEvents, err := reportingutil.PruneEvents(ctx, dlpEvents, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return true
	})
	if err != nil {
		return nil, errors.New("failed to prune events")
	}

	var events eventsBundle

	// Organize events according to their mode.
	for _, event := range prunedEvents {
		mode := event.WrappedEncryptedData.DlpPolicyEvent.Mode
		switch {
		case mode == "BLOCK":
			events.block = append(events.block, event)
		case mode == "REPORT":
			events.report = append(events.report, event)
		case mode == "WARN":
			events.warn = append(events.warn, event)
		case mode == "WARN_PROCEED":
			events.warnProceed = append(events.warnProceed, event)
		default:
			return nil, errors.New("unsupported event mode")
		}
	}

	return &events, nil

}

// privacyScreen returns whether the device has a privacy screen.
func privacyScreen() (bool, error) {

	c := hwdep.PrivacyScreen()

	dc := &frameworkprotocol.DeprecatedDeviceConfig{}
	features := &configpb.HardwareFeatures{
		PrivacyScreen: &configpb.HardwareFeatures_PrivacyScreen{
			Present: configpb.HardwareFeatures_PRESENT,
		},
	}

	satisfied, _, err := c.Satisfied(&frameworkprotocol.HardwareFeatures{HardwareFeatures: features, DeprecatedDeviceConfig: dc})
	if err != nil {
		return false, errors.Wrap(err, "error while evaluating condition")
	}

	return satisfied, nil

}

// validateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func validateEvents(reportingEnabled bool, mode dlp.Mode, action Action, events *eventsBundle) error {

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	var expectedEvents []reportingutil.InputEvent

	hasPrivacyScreen, err := privacyScreen()
	if err != nil {
		return errors.Wrap(err, "failed to check if privacy screen is available")
	}

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedEvents = events.block
			// If the device has a privacy screen, we will get an additional event when performing an action different from `PrivacyScreen`, which simply opens a web page to trigger the privacy screen.
			if hasPrivacyScreen && action != PrivacyScreen {
				expectedBlockEvents = 2
			} else {
				expectedBlockEvents = 1
			}
		case dlp.Mode_REPORT:
			expectedEvents = events.report
			expectedReportEvents = 1
		case dlp.Mode_WARN_CANCEL:
			expectedEvents = events.warn
			expectedWarnEvents = 1
		case dlp.Mode_WARN_PROCEED:
			expectedEvents = events.warn
			expectedWarnEvents = 1
			expectedWarnProceedEvents = 1
		}
	}

	if len(events.block) != expectedBlockEvents || len(events.report) != expectedReportEvents || len(events.warn) != expectedWarnEvents || len(events.warnProceed) != expectedWarnProceedEvents {
		return errors.Errorf("unexpected events = got %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Want %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events",
			len(events.block), len(events.report), len(events.report), len(events.warnProceed), expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents)
	}

	if !reportingEnabled || mode == dlp.Mode_ALLOW {
		return nil
	}

	if hasPrivacyScreen && mode == dlp.Mode_BLOCK && action != PrivacyScreen {
		if expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(PrivacyScreen))
		}
		if expectedEvents[1].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	} else {
		if expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	}

	if mode == dlp.Mode_WARN_PROCEED {
		if events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
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
	if _, err := service.EnrollAndLogin(ctx, &dlp.EnrollAndLoginRequest{
		Username:           username,
		Password:           password,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnableLacros:       params.BrowserType == dlp.BrowserType_LACROS,
		EnabledFeatures:    "EncryptedReportingPipeline",
	}); err != nil {
		s.Fatal("Remote call EnrollAndLogin() failed: ", err)
	}
	defer service.StopChrome(ctx, &empty.Empty{})

	// Create client instance of the Policy service just to retrieve the clientID.
	pc := ps.NewPolicyServiceClient(cl.Conn)

	// TODO(crbug.com/1376853): consider whether porting this method to the DataLeakPrevention service.
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
	case Screenshot:
		service.Screenshot(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	case PrivacyScreen:
		service.PrivacyScreen(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	}

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err := testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	events, err := retrieveEvents(ctx, customerID, APIKey, c.ClientId, testStartTime)
	if err != nil {
		s.Fatal("Failed to retrieve events: ", err)
	}

	if err := validateEvents(params.ReportingEnabled, params.Mode, action, events); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

	return nil

}
