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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DlpScreenshotReporting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test whether screenshot events are correctly reported for every restriction level",
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
				Name: "ash_warn_cancel_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingEnabledUsername,
					Password:         dlputil.RestrictionWarnReportingEnabledPassword,
					Mode:             dlp.Mode_WARN_CANCEL,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
				},
			},
			{
				Name: "ash_warn_cancel_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingDisabledUsername,
					Password:         dlputil.RestrictionWarnReportingDisabledPassword,
					Mode:             dlp.Mode_WARN_CANCEL,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: false,
				},
			},
			{
				Name: "lacros_warn_cancel_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingEnabledUsername,
					Password:         dlputil.RestrictionWarnReportingEnabledPassword,
					Mode:             dlp.Mode_WARN_CANCEL,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_warn_cancel_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingDisabledUsername,
					Password:         dlputil.RestrictionWarnReportingDisabledPassword,
					Mode:             dlp.Mode_WARN_CANCEL,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_warn_proceed_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingEnabledUsername,
					Password:         dlputil.RestrictionWarnReportingEnabledPassword,
					Mode:             dlp.Mode_WARN_PROCEED,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
				},
			},
			{
				Name: "ash_warn_proceed_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingDisabledUsername,
					Password:         dlputil.RestrictionWarnReportingDisabledPassword,
					Mode:             dlp.Mode_WARN_PROCEED,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: false,
				},
			},
			{
				Name: "lacros_warn_proceed_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingEnabledUsername,
					Password:         dlputil.RestrictionWarnReportingEnabledPassword,
					Mode:             dlp.Mode_WARN_PROCEED,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_warn_proceed_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionWarnReportingDisabledUsername,
					Password:         dlputil.RestrictionWarnReportingDisabledPassword,
					Mode:             dlp.Mode_WARN_PROCEED,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: false,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "ash_report_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionReportReportingEnabledUsername,
					Password:         dlputil.RestrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: true,
				},
			},
			{
				Name: "ash_report_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionReportReportingDisabledUsername,
					Password:         dlputil.RestrictionReportReportingDisabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_ASH,
					ReportingEnabled: false,
				},
			},
			{
				Name: "lacros_report_reporting_enabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionReportReportingEnabledUsername,
					Password:         dlputil.RestrictionReportReportingEnabledPassword,
					Mode:             dlp.Mode_REPORT,
					BrowserType:      dlp.BrowserType_LACROS,
					ReportingEnabled: true,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name: "lacros_report_reporting_disabled",
				Val: dlputil.TestParams{
					Username:         dlputil.RestrictionReportReportingDisabledUsername,
					Password:         dlputil.RestrictionReportReportingDisabledPassword,
					Mode:             dlp.Mode_REPORT,
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

func DlpScreenshotReporting(ctx context.Context, s *testing.State) {

	dlputil.ValidateActionReporting(ctx, s, dlputil.Screenshot)

}
