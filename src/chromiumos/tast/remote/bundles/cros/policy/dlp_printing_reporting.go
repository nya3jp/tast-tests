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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpPrintingReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test whether printing events are correctly reported for every restriction level",
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
		Params: dlputil.TestParameters,
	})
}

// dlpPolicyEventPrinting identifies printing events.
func dlpPolicyEventPrinting(event reportingutil.InputEvent, modeText string) bool {
	if w := event.WrappedEncryptedData; w != nil {
		if d := w.DlpPolicyEvent; d != nil && d.Restriction == "PRINTING" && (d.Mode == modeText || len(d.Mode) == 0) {
			return true
		}
	}
	return false
}

func DlpPrintingReporting(ctx context.Context, s *testing.State) {
	params := s.Param().(dlputil.TestParams)

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

	// Perform a printing action.
	service.Print(ctx, &dlp.ActionRequest{
		BrowserType: params.BrowserType,
		Mode:        params.Mode,
	})

	s.Log("Waiting 60 seconds to make sure events reach the server and are processed")
	if err = testing.Sleep(ctx, 60*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	blockEvents, reportEvents, warnEvents, warnProceedEvents, err := dlputil.RetrieveEvents(ctx, customerID, APIKey, c.ClientId, testStartTime, dlpPolicyEventPrinting)
	if err != nil {
		s.Fatal("Failed to retrieve events: ", err)
	}

	if err = dlputil.ValidateEvents(params.ReportingEnabled, params.Mode, blockEvents, reportEvents, warnEvents, warnProceedEvents); err != nil {
		s.Fatal("Failed to validate events: ", err)
	}

}
