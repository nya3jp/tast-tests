// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/remote/dlputil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	dlp "chromiumos/tast/services/cros/dlp"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const (
	// restrictionBlockReportingEnabledUsername is the path to the secret username having block restriction level for all components and reporting enabled.
	restrictionBlockReportingEnabledUsername = "dlp.restriction_level_block_reporting_enabled_username"

	// restrictionBlockReportingEnabledPassword is the path to the secret password having block restriction level for all components and reporting enabled.
	restrictionBlockReportingEnabledPassword = "dlp.restriction_level_block_reporting_enabled_password"

	// restrictionBlockReportingDisabledUsername is the path to the secret username having block restriction level for all components and reporting disabled.
	restrictionBlockReportingDisabledUsername = "dlp.restriction_level_block_reporting_disabled_username"

	// restrictionBlockReportingDisabledPassword is the path to the secret password having block restriction level for all components and reporting disabled.
	restrictionBlockReportingDisabledPassword = "dlp.restriction_level_block_reporting_disabled_password"

	// restrictionReportReportingEnabledUsername is the path to the secret username having report restriction level for all components and reporting enabled.
	restrictionReportReportingEnabledUsername = "dlp.restriction_level_report_reporting_enabled_username"

	// restrictionReportReportingEnabledPassword is the path to the secret password having report restriction level for all components and reporting enabled.
	restrictionReportReportingEnabledPassword = "dlp.restriction_level_report_reporting_enabled_password"

	// restrictionReportReportingDisabledUsername is the path to the secret username having report restriction level for all components and reporting disabled.
	restrictionReportReportingDisabledUsername = "dlp.restriction_level_report_reporting_disabled_username"

	// restrictionReportReportingDisabledPassword is the path to the secret password having report restriction level for all components and reporting disabled.
	restrictionReportReportingDisabledPassword = "dlp.restriction_level_report_reporting_disabled_password"

	// restrictionWarnReportingEnabledUsername is the path to the secret username having warn restriction level for all components and reporting enabled.
	restrictionWarnReportingEnabledUsername = "dlp.restriction_level_warn_reporting_enabled_username"

	// restrictionWarnReportingEnabledPassword is the path to the secret password having warn restriction level for all components and reporting enabled.
	restrictionWarnReportingEnabledPassword = "dlp.restriction_level_warn_reporting_enabled_password"

	// restrictionWarnReportingDisabledUsername is the path to the secret username having warn restriction level for all components and reporting disabled.
	restrictionWarnReportingDisabledUsername = "dlp.restriction_level_warn_reporting_disabled_username"

	// restrictionWarnReportingDisabledPassword is the path to the secret password having warn restriction level for all components and reporting disabled.
	restrictionWarnReportingDisabledPassword = "dlp.restriction_level_warn_reporting_disabled_password"

	// restrictionAllowReportingEnabledUsername is the path to the secret username having allow restriction level for all components and reporting enabled.
	restrictionAllowReportingEnabledUsername = "dlp.restriction_level_allow_reporting_enabled_username"

	// restrictionAllowReportingEnabledPassword is the path to the secret password having allow restriction level for all components and reporting enabled.
	restrictionAllowReportingEnabledPassword = "dlp.restriction_level_allow_reporting_enabled_password"

	// restrictionAllowReportingDisabledUsername is the path to the secret username having allow restriction level for all components and reporting disabled.
	restrictionAllowReportingDisabledUsername = "dlp.restriction_level_allow_reporting_disabled_username"

	// restrictionAllowReportingDisabledPassword is the path to the secret password having allow restriction level for all components and reporting disabled.
	restrictionAllowReportingDisabledPassword = "dlp.restriction_level_allow_reporting_disabled_password"
)

// testParams contains parameters for testing different DLP configurations.
type testParams struct {
	Username         string          // username for Chrome enrollment
	Password         string          // password for Chrome enrollment
	Mode             dlp.Mode        // mode of the applied restriction
	BrowserType      dlp.BrowserType // which browser the test should use
	ReportingEnabled bool            // test should expect reporting to be enabled
	Action           dlputil.Action  // which action the test should use
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests DLP actions and check whether the correct events are generated, sent, and received from the server side",
		Contacts: []string{
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
				Name: "ash_clipboard_copy_paste_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
					Action:           dlputil.ClipboardCopyPaste,
				},
			},
			{
				Name: "lacros_clipboard_copy_paste_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
					Action:           dlputil.ClipboardCopyPaste,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_printing_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
					Action:           dlputil.Printing,
				},
			},
			{
				Name: "lacros_printing_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
					Action:           dlputil.Printing,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_screenshot_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
					Action:           dlputil.Screenshot,
				},
			},
			{
				Name: "lacros_screenshot_report_reporting_enabled",
				Val: testParams{
					Username:         restrictionReportReportingEnabledUsername,
					Password:         restrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
					Action:           dlputil.Screenshot,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func DlpReporting(ctx context.Context, s *testing.State) {

	params := s.Param().(testParams)

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

	switch params.Action {
	case dlputil.ClipboardCopyPaste:
		service.ClipboardCopyPaste(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	case dlputil.Printing:
		service.Print(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	case dlputil.Screenshot:
		service.Screenshot(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
			Mode:        params.Mode,
		})
	}

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err := testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	events, err := dlputil.RetrieveEvents(ctx, customerID, APIKey, c.ClientId, testStartTime)
	if err != nil {
		s.Fatal("Failed to retrieve events: ", err)
	}

	if err := dlputil.ValidateEvents(params.ReportingEnabled, params.Mode, params.Action, events); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

}
