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

func DlpPrintingReporting(ctx context.Context, s *testing.State) {

	dlputil.DlpActionReporting(ctx, s, dlputil.Printing)

}
