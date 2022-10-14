// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/remote/dlputil"
	"chromiumos/tast/remote/reportingutil"
	dlp "chromiumos/tast/services/cros/dlp"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpPrivacyScreenReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test whether privacy screen events are correctly reported for supported restriction levels",
		Contacts: []string{
			"chromeos-dlp@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		HardwareDeps: hwdep.D(hwdep.PrivacyScreen()),
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
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionBlockReportingEnabledUsername,
					Password:         dlputil.RestrictionBlockReportingEnabledPassword,
					Mode:             dlp.Mode_BLOCK,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
				},
			},
			{
				Name: "ash_block_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionBlockReportingDisabledUsername,
					Password:         dlputil.RestrictionBlockReportingDisabledPassword,
					Mode:             dlp.Mode_BLOCK,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: false,
				},
			},
			{
				Name: "lacros_block_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionBlockReportingEnabledUsername,
					Password:         dlputil.RestrictionBlockReportingEnabledPassword,
					Mode:             dlp.Mode_BLOCK,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_block_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionBlockReportingDisabledUsername,
					Password:         dlputil.RestrictionBlockReportingDisabledPassword,
					Mode:             dlp.Mode_BLOCK,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_allow_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionAllowReportingEnabledUsername,
					Password:         dlputil.RestrictionAllowReportingEnabledPassword,
					Mode:             dlp.Mode_ALLOW,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
				},
			},
			{
				Name: "ash_allow_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionAllowReportingDisabledUsername,
					Password:         dlputil.RestrictionAllowReportingDisabledPassword,
					Mode:             dlp.Mode_ALLOW,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: false,
				},
			},
			{
				Name: "lacros_allow_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionAllowReportingEnabledUsername,
					Password:         dlputil.RestrictionAllowReportingEnabledPassword,
					Mode:             dlp.Mode_ALLOW,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_allow_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionAllowReportingDisabledUsername,
					Password:         dlputil.RestrictionAllowReportingDisabledPassword,
					Mode:             dlp.Mode_ALLOW,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func DlpPrivacyScreenReporting(ctx context.Context, s *testing.State) {

	dlputil.ValidateActionReporting(ctx, s, dlputil.PrivacyScreen)

}
