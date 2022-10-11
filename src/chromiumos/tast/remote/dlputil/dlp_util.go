// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlputil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/reportingutil"
	dlp "chromiumos/tast/services/cros/dlp"
)

// RestrictionBlockReportingEnabledUsername is the path to the secret username having block restriction level for all components and reporting enabled.
const RestrictionBlockReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_block_reporting_enabled_username"

// RestrictionBlockReportingEnabledPassword is the path to the secret password having block restriction level for all components and reporting enabled.
const RestrictionBlockReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_block_reporting_enabled_password"

// RestrictionBlockReportingDisabledUsername is the path to the secret username having block restriction level for all components and reporting disabled.
const RestrictionBlockReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_block_reporting_disabled_username"

// RestrictionBlockReportingDisabledPassword is the path to the secret password having block restriction level for all components and reporting disabled.
const RestrictionBlockReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_block_reporting_disabled_password"

// RestrictionReportReportingEnabledUsername is the path to the secret username having report restriction level for all components and reporting enabled.
const RestrictionReportReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_report_reporting_enabled_username"

// RestrictionReportReportingEnabledPassword is the path to the secret password having report restriction level for all components and reporting enabled.
const RestrictionReportReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_report_reporting_enabled_password"

// RestrictionReportReportingDisabledUsername is the path to the secret username having report restriction level for all components and reporting disabled.
const RestrictionReportReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_report_reporting_disabled_username"

// RestrictionReportReportingDisabledPassword is the path to the secret password having report restriction level for all components and reporting disabled.
const RestrictionReportReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_report_reporting_disabled_password"

// RestrictionWarnReportingEnabledUsername is the path to the secret username having warn restriction level for all components and reporting enabled.
const RestrictionWarnReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_warn_reporting_enabled_username"

// RestrictionWarnReportingEnabledPassword is the path to the secret password having warn restriction level for all components and reporting enabled.
const RestrictionWarnReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_warn_reporting_enabled_password"

// RestrictionWarnReportingDisabledUsername is the path to the secret username having warn restriction level for all components and reporting disabled.
const RestrictionWarnReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_warn_reporting_disabled_username"

// RestrictionWarnReportingDisabledPassword is the path to the secret password having warn restriction level for all components and reporting disabled.
const RestrictionWarnReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_warn_reporting_disabled_password"

// RestrictionAllowReportingEnabledUsername is the path to the secret username having allow restriction level for all components and reporting enabled.
const RestrictionAllowReportingEnabledUsername = "policy.DlpClipboardReporting.restriction_level_allow_reporting_enabled_username"

// RestrictionAllowReportingEnabledPassword is the path to the secret password having allow restriction level for all components and reporting enabled.
const RestrictionAllowReportingEnabledPassword = "policy.DlpClipboardReporting.restriction_level_allow_reporting_enabled_password"

// RestrictionAllowReportingDisabledUsername is the path to the secret username having allow restriction level for all components and reporting disabled.
const RestrictionAllowReportingDisabledUsername = "policy.DlpClipboardReporting.restriction_level_allow_reporting_disabled_username"

// RestrictionAllowReportingDisabledPassword is the path to the secret password having allow restriction level for all components and reporting disabled.
const RestrictionAllowReportingDisabledPassword = "policy.DlpClipboardReporting.restriction_level_allow_reporting_disabled_password"

// retrieveEvents returns events having a timestamp greater than `testStartTime` with the given `clientID` and satisfying `correctEventType`.
func retrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time, correctEventType func(reportingutil.InputEvent, string) bool, modeText string) ([]reportingutil.InputEvent, error) {

	events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	// Fatal error occurred while looking up events.
	if err != nil {
		return nil, errors.Wrap(err, "failed to look up events")
	}

	prunedEvents, err := reportingutil.PruneEvents(ctx, events, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return correctEventType(e, modeText)
	})
	if err != nil {
		return nil, errors.New("failed to prune events")
	}

	return prunedEvents, nil

}

// RetrieveEvents returns events for every restriction level having a timestamp greater than `testStartTime` with the given `clientID` and satisfying `correctEventType`.
func RetrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time, correctEventType func(reportingutil.InputEvent, string) bool) ([]reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, error) {

	blockEvents, err := retrieveEvents(ctx, customerID, APIKey, clientID, testStartTime, correctEventType, "BLOCK")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "error while retrieving BLOCK events")
	}
	reportEvents, err := retrieveEvents(ctx, customerID, APIKey, clientID, testStartTime, correctEventType, "REPORT")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "error while retrieving REPORT events")
	}
	warnEvents, err := retrieveEvents(ctx, customerID, APIKey, clientID, testStartTime, correctEventType, "WARN")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "error while retrieving WARN events")
	}
	warnProceedEvents, err := retrieveEvents(ctx, customerID, APIKey, clientID, testStartTime, correctEventType, "WARN_PROCEED")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "error while retrieving WARN_PROCEED events")
	}

	return blockEvents, reportEvents, warnEvents, warnProceedEvents, nil

}

// ValidateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func ValidateEvents(reportingEnabled bool, mode dlp.Mode, blockEvents, reportEvents, warnEvents, warnProceedEvents []reportingutil.InputEvent) error {

	expectedNumRetrievedEvents := 1
	if !reportingEnabled || mode == dlp.Mode_ALLOW {
		expectedNumRetrievedEvents = 0
	}

	switch mode {
	case dlp.Mode_BLOCK:
		if len(blockEvents) != expectedNumRetrievedEvents {
			return errors.Errorf("Expecting %d BLOCK events, got %d instead", expectedNumRetrievedEvents, len(blockEvents))
		}
		if len(reportEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			return errors.Errorf("Expecting 0 REPORT, 0 WARN, and 0 WARN_PROCEED events. Got %d REPORT, %d WARN, and % WARN_PROCEED events instead",
				len(reportEvents), len(reportEvents), len(warnProceedEvents))
		}
	case dlp.Mode_REPORT:
		if len(reportEvents) != expectedNumRetrievedEvents {
			return errors.Errorf("Expecting %d REPORT events, got %d instead", expectedNumRetrievedEvents, len(reportEvents))
		}
		if len(blockEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			return errors.Errorf("Expecting 0 BLOCK, 0 WARN, and 0 WARN_PROCEED events. Got %d BLOCK, %d WARN, and %d WARN_PROCEED events instead",
				len(blockEvents), len(warnEvents), len(warnProceedEvents))
		}
	case dlp.Mode_WARN_CANCEL:
		if len(warnEvents) != expectedNumRetrievedEvents {
			return errors.Errorf("Expecting %d WARN events, got %d instead", expectedNumRetrievedEvents, len(warnEvents))
		}
		if len(blockEvents) != 0 || len(reportEvents) != 0 || len(warnProceedEvents) != 0 {
			return errors.Errorf("Expecting 0 BLOCK, 0 REPORT, and 0 WARN_PROCEED events. Got %d BLOCK, %d REPORT, and %d WARN_PROCEED events instead",
				len(blockEvents), len(reportEvents), len(warnProceedEvents))
		}
	case dlp.Mode_WARN_PROCEED:
		if len(warnEvents) != expectedNumRetrievedEvents || len(warnProceedEvents) != expectedNumRetrievedEvents {
			return errors.Errorf("Expecting %d WARN and %d WARN_PROCEED events. Got %d and %d instead",
				expectedNumRetrievedEvents, expectedNumRetrievedEvents, len(warnEvents), len(warnProceedEvents))
		}
		if len(blockEvents) != 0 || len(reportEvents) != 0 {
			return errors.Errorf("Expecting 0 BLOCK and 0 REPORT events. Got %d BLOCK and %d REPORT events instead",
				len(blockEvents), len(reportEvents))
		}
	case dlp.Mode_ALLOW:
		if len(reportEvents) != 0 || len(warnEvents) != 0 || len(warnProceedEvents) != 0 {
			return errors.Errorf("Expecting 0 BLOCK, 0 REPORT, 0 WARN, and 0 WARN_PROCEED events. Got %d BLOCK, %d REPORT, %d WARN, and % WARN_PROCEED events instead",
				len(blockEvents), len(reportEvents), len(reportEvents), len(warnProceedEvents))

		}
	}

	return nil
}
