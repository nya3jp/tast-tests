// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlputil

import (
	"context"
	"fmt"
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

// Action represents the supported DLP actions.
type Action int

const (
	// ClipboardCopyPaste identifies a clipboard copy and paste action.
	ClipboardCopyPaste Action = iota
)

func (action Action) String() string {
	switch action {
	case ClipboardCopyPaste:
		return "CLIPBOARD"
	default:
		return fmt.Sprintf("String() not defined for Action %d", int(action))
	}
}

// RetrieveEvents returns events for every restriction level having a timestamp greater than `testStartTime` with the given `clientID` and satisfying `correctEventType`.
func RetrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time) ([]reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, []reportingutil.InputEvent, error) {

	// Retrieve all DLP events stored in the server side.
	events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to look up events")
	}

	// Reduce events to those associate to `clientID` and starting after `testStartTime`.
	prunedEvents, err := reportingutil.PruneEvents(ctx, events, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return true
	})
	if err != nil {
		return nil, nil, nil, nil, errors.New("failed to prune events")
	}

	// Organize events according to their mode.
	var blockEvents []reportingutil.InputEvent
	var reportEvents []reportingutil.InputEvent
	var warnEvents []reportingutil.InputEvent
	var warnProceedEvents []reportingutil.InputEvent

	for _, event := range prunedEvents {
		mode := event.WrappedEncryptedData.DlpPolicyEvent.Mode
		switch {
		case mode == "BLOCK":
			blockEvents = append(blockEvents, event)
		case mode == "REPORT":
			reportEvents = append(reportEvents, event)
		case mode == "WARN":
			warnEvents = append(warnEvents, event)
		case mode == "WARN_PROCEED":
			warnProceedEvents = append(warnProceedEvents, event)
		default:
			return nil, nil, nil, nil, errors.New("unsupported event mode")
		}
	}

	return blockEvents, reportEvents, warnEvents, warnProceedEvents, nil

}

// ValidateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func ValidateEvents(reportingEnabled bool, mode dlp.Mode, action Action, blockEvents, reportEvents, warnEvents, warnProceedEvents []reportingutil.InputEvent) error {

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	var expectedEvents *[]reportingutil.InputEvent

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedEvents = &blockEvents
			expectedBlockEvents = 1
		case dlp.Mode_REPORT:
			expectedEvents = &reportEvents
			expectedReportEvents = 1
		case dlp.Mode_WARN_CANCEL:
			expectedEvents = &warnEvents
			expectedWarnEvents = 1
		case dlp.Mode_WARN_PROCEED:
			expectedEvents = &warnEvents
			expectedWarnEvents = 1
			expectedWarnProceedEvents = 1
		}
	}

	if len(blockEvents) != expectedBlockEvents ||
		len(reportEvents) != expectedReportEvents ||
		len(warnEvents) != expectedWarnEvents ||
		len(warnProceedEvents) != expectedWarnProceedEvents {
		return errors.Errorf("Expecting %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Got %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events instead",
			expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents,
			len(blockEvents), len(reportEvents), len(reportEvents), len(warnProceedEvents))
	}

	if (*expectedEvents)[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
		return errors.Errorf("Expecting %v restriction, got %v instead", Action.String(action), (*expectedEvents)[0].WrappedEncryptedData.DlpPolicyEvent.Restriction)
	}

	if mode == dlp.Mode_WARN_PROCEED {
		if warnProceedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("Expecting %v restriction, got %v instead", Action.String(action), warnProceedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction)
		}
	}

	return nil
}
