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
	"chromiumos/tast/testing"
)

// Action represents the supported DLP actions.
type Action int

const (
	// ClipboardCopyPaste identifies a clipboard copy and paste action.
	ClipboardCopyPaste Action = iota
	// Printing identifies a printing action.
	Printing
)

// String returns a string representation of `Action`.
func (action Action) String() string {
	switch action {
	case ClipboardCopyPaste:
		return "CLIPBOARD"
	case Printing:
		return "PRINTING"
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
	dlpEvents, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, customerID, clientID, APIKey, "DLP_EVENTS", testStartTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to look up events")
	}

	// Reduce events to those associate to `clientID` and starting after `testStartTime`.
	prunedEvents, err := reportingutil.PruneEvents(ctx, dlpEvents, func(e reportingutil.InputEvent) bool {
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
func ValidateReportEvents(ctx context.Context, action Action, events *EventsBundle) error {

	var firstErr error

	if want := 0; len(events.block) != want {
		testing.ContextLogf(ctx, "Unexpected BLOCK events = got %d, want %d", len(events.block), want)
		firstErr = errors.Errorf("unexpected number of BLOCK events = got %d, want %d", len(events.block), want)
	}

	if want := 1; len(events.report) != want {
		testing.ContextLogf(ctx, "Unexpected REPORT events = got %d, want %d", len(events.report), want)
		if firstErr == nil {
			firstErr = errors.Errorf("unexpected number of REPORT events = got %d, want %d", len(events.report), want)
		}
	}

	if want := 0; len(events.warn) != want {
		testing.ContextLogf(ctx, "Unexpected WARN events = got %d, want %d", len(events.warn), want)
		if firstErr == nil {
			firstErr = errors.Errorf("unexpected number of WARN events = got %d, want %d", len(events.warn), want)
		}
	}

	if want := 0; len(events.warnProceed) != want {
		testing.ContextLogf(ctx, "Unexpected WARN_PROCEED events = got %d, want %d", len(events.warnProceed), want)
		if firstErr == nil {
			firstErr = errors.Errorf("unexpected number of WARN_PROCEED events = got %d, want %d", len(events.warnProceed), want)
		}
	}

	if len(events.report) > 0 && events.report[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
		testing.ContextLogf(ctx, "Unexpected restriction = got %v, want %v", events.report[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		if firstErr == nil {
			firstErr = errors.Errorf("unexpected restriction = got %v, want %v", events.report[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	}

	return firstErr
}
