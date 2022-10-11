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

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedBlockEvents = 1
		case dlp.Mode_REPORT:
			expectedReportEvents = 1
		case dlp.Mode_WARN_CANCEL:
			expectedWarnEvents = 1
		case dlp.Mode_WARN_PROCEED:
			expectedWarnEvents = 1
			expectedWarnProceedEvents = 1
		}
	}

	if len(blockEvents) != expectedBlockEvents || len(reportEvents) != expectedReportEvents || len(warnEvents) != expectedWarnEvents || len(warnProceedEvents) != expectedWarnProceedEvents {
		return errors.Errorf("Expecting %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Got %d BLOCK, %d REPORT, %d WARN, and % WARN_PROCEED events instead",
			expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents,
			len(blockEvents), len(reportEvents), len(reportEvents), len(warnProceedEvents))
	}

	return nil
}
