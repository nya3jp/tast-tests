// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlputil

import (
	"context"
	"fmt"
	"time"

	configpb "go.chromium.org/chromiumos/config/go/api"

	"chromiumos/tast/errors"
	frameworkprotocol "chromiumos/tast/framework/protocol"
	"chromiumos/tast/remote/reportingutil"
	dlp "chromiumos/tast/services/cros/dlp"
	"chromiumos/tast/testing/hwdep"
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
	// PrivacyScreen identifies a privacy screen enforcement.
	PrivacyScreen
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
	case PrivacyScreen:
		return "EPRIVACY"
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

// privacyScreen returns whether the device has a privacy screen.
func privacyScreen() (bool, error) {

	c := hwdep.PrivacyScreen()

	dc := &frameworkprotocol.DeprecatedDeviceConfig{}
	features := &configpb.HardwareFeatures{
		PrivacyScreen: &configpb.HardwareFeatures_PrivacyScreen{
			Present: configpb.HardwareFeatures_PRESENT,
		},
	}

	satisfied, _, err := c.Satisfied(&frameworkprotocol.HardwareFeatures{HardwareFeatures: features, DeprecatedDeviceConfig: dc})
	if err != nil {
		return false, errors.Wrap(err, "error while evaluating condition")
	}

	return satisfied, nil

}

// ValidateEvents checks whether events array contains the correct events according to `reportingEnabled` and `mode`.
func ValidateEvents(reportingEnabled bool, mode dlp.Mode, action Action, events *EventsBundle) error {

	expectedBlockEvents := 0
	expectedReportEvents := 0
	expectedWarnEvents := 0
	expectedWarnProceedEvents := 0

	var expectedEvents []reportingutil.InputEvent

	hasPrivacyScreen, err := privacyScreen()
	if err != nil {
		return errors.Wrap(err, "failed to check if privacy screen is available")
	}

	if reportingEnabled {
		switch mode {
		case dlp.Mode_BLOCK:
			expectedEvents = events.block
			// If the device has a privacy screen, we will get an additional event when performing an action different from `PrivacyScreen`, which simply opens a web page to trigger the privacy screen.
			if hasPrivacyScreen && action != PrivacyScreen {
				expectedBlockEvents = 2
			} else {
				expectedBlockEvents = 1
			}
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

	if !reportingEnabled || mode == dlp.Mode_ALLOW {
		return nil
	}

	if hasPrivacyScreen && mode == dlp.Mode_BLOCK && action != PrivacyScreen {
		if expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(PrivacyScreen))
		}
		if expectedEvents[1].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	} else {
		if expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", expectedEvents[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	}

	if mode == dlp.Mode_WARN_PROCEED {
		if events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction != Action.String(action) {
			return errors.Errorf("unexpected restriction = got %v, want %v", events.warnProceed[0].WrappedEncryptedData.DlpPolicyEvent.Restriction, Action.String(action))
		}
	}

	return nil
}
