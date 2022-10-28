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

	// restrictionReportReportingEnabledUsername is the path to the secret username having report restriction level for all components and reporting enabled.
	restrictionReportReportingEnabledUsername = "dlp.restriction_level_report_reporting_enabled_username"

	// restrictionReportReportingEnabledPassword is the path to the secret password having report restriction level for all components and reporting enabled.
	restrictionReportReportingEnabledPassword = "dlp.restriction_level_report_reporting_enabled_password"
)

// testParams contains parameters for testing different DLP configurations.
type testParams struct {
	Username    string          // username for Chrome enrollment
	Password    string          // password for Chrome enrollment
	BrowserType dlp.BrowserType // which browser the test should use
	Action      dlputil.Action  // which action the test should use
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests DLP actions in report mode and check whether the correct events are generated, sent, and received from the server side",
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
			restrictionReportReportingEnabledUsername,
			restrictionReportReportingEnabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
		Params: []testing.Param{
			{
				Name: "ash_screenshare",
				Val: testParams{
					Username:    restrictionReportReportingEnabledUsername,
					Password:    restrictionReportReportingEnabledPassword,
					BrowserType: dlp.BrowserType_ASH,
					Action:      dlputil.Screenshare,
				},
			},
			// {
			// 	Name: "ash_clipboard_copy_paste",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_ASH,
			// 		Action:      dlputil.ClipboardCopyPaste,
			// 	},
			// },
			// {
			// 	Name: "lacros_clipboard_copy_paste",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_LACROS,
			// 		Action:      dlputil.ClipboardCopyPaste,
			// 	},
			// 	ExtraSoftwareDeps: []string{"lacros"},
			// },
			// {
			// 	Name: "ash_print",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_ASH,
			// 		Action:      dlputil.Printing,
			// 	},
			// },
			// {
			// 	Name: "lacros_print",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_LACROS,
			// 		Action:      dlputil.Printing,
			// 	},
			// 	ExtraSoftwareDeps: []string{"lacros"},
			// },
			// {
			// 	Name: "ash_screenshot",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_ASH,
			// 		Action:      dlputil.Screenshot,
			// 	},
			// },
			// {
			// 	Name: "lacros_screenshot",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_LACROS,
			// 		Action:      dlputil.Screenshot,
			// 	},
			// 	ExtraSoftwareDeps: []string{"lacros"},
			// },
			// {
			// 	Name: "ash_screenshare",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_ASH,
			// 		Action:      dlputil.Screenshare,
			// 	},
			// },
			// {
			// 	Name: "lacros_screenshare",
			// 	Val: testParams{
			// 		Username:    restrictionReportReportingEnabledUsername,
			// 		Password:    restrictionReportReportingEnabledPassword,
			// 		BrowserType: dlp.BrowserType_LACROS,
			// 		Action:      dlputil.Screenshare,
			// 	},
			// 	ExtraSoftwareDeps: []string{"lacros"},
			// },
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
		})
	case dlputil.Printing:
		service.Print(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
		})
	case dlputil.Screenshot:
		service.Screenshot(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
		})
	case dlputil.Screenshare:
		service.Screenshare(ctx, &dlp.ActionRequest{
			BrowserType: params.BrowserType,
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

	if err := dlputil.ValidateReportEvents(params.Action, events); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

}
