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

// Action represents the supported DLP actions.
type Action int

const (
	// ClipboardCopyPaste identifies a clipboard copy and paste action.
	ClipboardCopyPaste Action = iota
	// Printing identifies a printing action.
	Printing
	// Screenshot identifies a screenshot action.
	Screenshot
)

// String returns a string representation of `Action`.
func (action Action) String() string {
	switch action {
	case ClipboardCopyPaste:
		return "CLIPBOARD"
	case Printing:
		return "PRINTING"
	case Screenshot:
		return "SCREENSHOT"
	default:
		return fmt.Sprintf("String() not defined for Action %d", int(action))
	}
}

// EventsBundle contains an events vector per restriction level and it is populated by `retrieveEvents`.
type EventsBundle struct {
	block       []reportingutil.InputEvent
	report      []reportingutil.InputEvent
	warn        []reportingutil.InputEvent
	warnProceed []reportingutil.InputEvent
}

// RetrieveEvents returns events for every restriction level having a timestamp greater than `testStartTime` with the given `clientID`.
func RetrieveEvents(ctx context.Context, customerID, APIKey, clientID string, testStartTime time.Time) (*EventsBundle, error) {

	// Retrieve all DLP events stored in the server side.
	dlpEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, APIKey, "DLP_EVENTS")
	if err != nil {
		return nil, errors.Wrap(err, "failed to look up events")
	}

	// Reduce events to those associate to `clientID` and starting after `testStartTime`.
	prunedEvents, err := reportingutil.PruneEvents(ctx, dlpEvents, clientID, testStartTime, func(e reportingutil.InputEvent) bool {
		return true
	})
	if err != nil {
		return nil, errors.New("failed to prune events")
	}

	var events EventsBundle

	// Organize events according to their mode.
	for _, event := range prunedEvents {
		mode := event.WrappedEncryptedData.DlpPolicyEvent.Mode
		switch {
		case mode == "BLOCK":
			events.block = append(events.block, event)
		case mode == "REPORT":
			events.report = append(events.report, event)
		case mode == "WARN":
			events.warn = append(events.warn, event)
		case mode == "WARN_PROCEED":
			events.warnProceed = append(events.warnProceed, event)
		default:
			return nil, errors.New("unsupported event mode")
		}
	}

	return &events, nil

}

// ValidateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func ValidateEvents(reportingEnabled bool, mode dlp.Mode, action Action, events *EventsBundle) error {

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	var expectedEvents []reportingutil.InputEvent

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedEvents = events.block
			expectedBlockEvents = 1
		case dlp.Mode_REPORT:
			expectedEvents = events.report
			expectedReportEvents = 1
		case dlp.Mode_WARN_CANCEL:
			expectedEvents = events.warn
			expectedWarnEvents = 1
		case dlp.Mode_WARN_PROCEED:
			expectedEvents = events.warn
			expectedWarnEvents = 1
			expectedWarnProceedEvents = 1
		}
	}

	if len(events.block) != expectedBlockEvents || len(events.report) != expectedReportEvents || len(events.warn) != expectedWarnEvents || len(events.warnProceed) != expectedWarnProceedEvents {
		return errors.Errorf("unexpected events = got %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Want %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events",
			len(events.block), len(events.report), len(events.report), len(events.warnProceed), expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents)
	}

	if len(expectedEvents) > 0 && expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
		return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
	}

	if mode == dlp.Mode_WARN_PROCEED {
		if len(events.warnProceed) > 0 && events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	}

	return nil
}
