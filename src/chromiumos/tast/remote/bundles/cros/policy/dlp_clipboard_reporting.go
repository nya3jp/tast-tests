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

// A struct containing parameters for different clipboard tests.
type clipboardTestParams struct {
	username         string          // username for Chrome enrollment
	password         string          // password for Chrome enrollment
	mode             dlp.Mode        // mode of the applied restriction
	browserType      dlp.BrowserType // which browser the test should use
	reportingEnabled bool            // test should expect reporting to be enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpClipboardReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test whether clipboard copy and paste events are correctly reported for every restriction level",
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
			dlputil.RestrictionBlockReportingEnabledUsername,
			dlputil.RestrictionBlockReportingEnabledPassword,
			dlputil.RestrictionBlockReportingDisabledUsername,
			dlputil.RestrictionBlockReportingDisabledPassword,
			dlputil.RestrictionWarnReportingEnabledUsername,
			dlputil.RestrictionWarnReportingEnabledPassword,
			dlputil.RestrictionWarnReportingDisabledUsername,
			dlputil.RestrictionWarnReportingDisabledPassword,
			dlputil.RestrictionReportReportingEnabledUsername,
			dlputil.RestrictionReportReportingEnabledPassword,
			dlputil.RestrictionReportReportingDisabledUsername,
			dlputil.RestrictionReportReportingDisabledPassword,
			dlputil.RestrictionAllowReportingEnabledUsername,
			dlputil.RestrictionAllowReportingEnabledPassword,
			dlputil.RestrictionAllowReportingDisabledUsername,
			dlputil.RestrictionAllowReportingDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
		Params: []testing.Param{
			{
				Name: "ash_block_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionBlockReportingEnabledUsername,
					password:         dlputil.RestrictionBlockReportingEnabledPassword,
					mode:             dlp.Mode_BLOCK,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: true,
				},
			},
			{
				Name: "ash_block_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionBlockReportingDisabledUsername,
					password:         dlputil.RestrictionBlockReportingDisabledPassword,
					mode:             dlp.Mode_BLOCK,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: false,
				},
			},
			{
				Name: "lacros_block_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionBlockReportingEnabledUsername,
					password:         dlputil.RestrictionBlockReportingEnabledPassword,
					mode:             dlp.Mode_BLOCK,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_block_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionBlockReportingDisabledUsername,
					password:         dlputil.RestrictionBlockReportingDisabledPassword,
					mode:             dlp.Mode_BLOCK,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_warn_cancel_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingEnabledUsername,
					password:         dlputil.RestrictionWarnReportingEnabledPassword,
					mode:             dlp.Mode_WARN_CANCEL,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: true,
				},
			},
			{
				Name: "ash_warn_cancel_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingDisabledUsername,
					password:         dlputil.RestrictionWarnReportingDisabledPassword,
					mode:             dlp.Mode_WARN_CANCEL,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: false,
				},
			},
			{
				Name: "lacros_warn_cancel_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingEnabledUsername,
					password:         dlputil.RestrictionWarnReportingEnabledPassword,
					mode:             dlp.Mode_WARN_CANCEL,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_warn_cancel_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingDisabledUsername,
					password:         dlputil.RestrictionWarnReportingDisabledPassword,
					mode:             dlp.Mode_WARN_CANCEL,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_warn_proceed_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingEnabledUsername,
					password:         dlputil.RestrictionWarnReportingEnabledPassword,
					mode:             dlp.Mode_WARN_PROCEED,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: true,
				},
			},
			{
				Name: "ash_warn_proceed_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingDisabledUsername,
					password:         dlputil.RestrictionWarnReportingDisabledPassword,
					mode:             dlp.Mode_WARN_PROCEED,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: false,
				},
			},
			{
				Name: "lacros_warn_proceed_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingEnabledUsername,
					password:         dlputil.RestrictionWarnReportingEnabledPassword,
					mode:             dlp.Mode_WARN_PROCEED,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_warn_proceed_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionWarnReportingDisabledUsername,
					password:         dlputil.RestrictionWarnReportingDisabledPassword,
					mode:             dlp.Mode_WARN_PROCEED,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_report_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionReportReportingEnabledUsername,
					password:         dlputil.RestrictionReportReportingEnabledPassword,
					mode:             dlp.Mode_REPORT,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: true,
				},
			},
			{
				Name: "ash_report_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionReportReportingDisabledUsername,
					password:         dlputil.RestrictionReportReportingDisabledPassword,
					mode:             dlp.Mode_REPORT,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: false,
				},
			},
			{
				Name: "lacros_report_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionReportReportingEnabledUsername,
					password:         dlputil.RestrictionReportReportingEnabledPassword,
					mode:             dlp.Mode_REPORT,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_report_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionReportReportingDisabledUsername,
					password:         dlputil.RestrictionReportReportingDisabledPassword,
					mode:             dlp.Mode_REPORT,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_allow_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionAllowReportingEnabledUsername,
					password:         dlputil.RestrictionAllowReportingEnabledPassword,
					mode:             dlp.Mode_ALLOW,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: true,
				},
			},
			{
				Name: "ash_allow_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionAllowReportingDisabledUsername,
					password:         dlputil.RestrictionAllowReportingDisabledPassword,
					mode:             dlp.Mode_ALLOW,
					browserType:      dlp.BrowserType_ASH,
					reportingEnabled: false,
				},
			},
			{
				Name: "lacros_allow_reporting_enabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionAllowReportingEnabledUsername,
					password:         dlputil.RestrictionAllowReportingEnabledPassword,
					mode:             dlp.Mode_ALLOW,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_allow_reporting_disabled",
				Val: clipboardTestParams{
					username:         dlputil.RestrictionAllowReportingDisabledUsername,
					password:         dlputil.RestrictionAllowReportingDisabledPassword,
					mode:             dlp.Mode_ALLOW,
					browserType:      dlp.BrowserType_LACROS,
					reportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
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
		EnableLacros:       params.browserType == dlp.BrowserType_LACROS,
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

	// Perform a copy and paste action.
	service.ClipboardCopyPaste(ctx, &dlp.ClipboardCopyPasteRequest{
		BrowserType: params.browserType,
		Mode:        params.mode,
	})

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err = testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	blockEvents, reportEvents, warnEvents, warnProceedEvents, err := dlputil.RetrieveEvents(ctx, customerID, APIKey, c.ClientId, testStartTime)
	if err != nil {
		s.Fatal("Failed to retrieve events: ", err)
	}

	if err = dlputil.ValidateEvents(params.reportingEnabled, params.mode, dlputil.ClipboardCopyPaste, blockEvents, reportEvents, warnEvents, warnProceedEvents); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

}
