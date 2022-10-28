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
	// Screenshare identifies a screenshare action.
	Screenshare
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
	case Screenshare:
		return "SCREENCAST"
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

// ValidateReportEvents checks whether events array contains the correct events.
func ValidateReportEvents(action Action, events *EventsBundle) error {

	// We are only testing report mode
	expectedBlockEvents := 0
	expectedReportEvents := 1
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	if len(events.block) != expectedBlockEvents || len(events.report) != expectedReportEvents || len(events.warn) != expectedWarnEvents || len(events.warnProceed) != expectedWarnProceedEvents {
		return errors.Errorf("unexpected events = got %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events. Want %d BLOCK, %d REPORT, %d WARN, and %d WARN_PROCEED events",
			len(events.block), len(events.report), len(events.warn), len(events.warnProceed),
			expectedBlockEvents, expectedReportEvents, expectedWarnEvents, expectedWarnProceedEvents)
	}

	if len(events.report) > 0 && events.report[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
		return errors.Errorf("unexpected restriction = got %v, want %v", events.report[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
	}

	return nil
}
