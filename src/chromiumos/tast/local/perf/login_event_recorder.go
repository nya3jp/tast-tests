// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"sort"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// LoginEventRecorderData holds the collected login events.
type LoginEventRecorderData struct {
	Name                       string `json:"name"`
	MicrosecnodsSinceUnixEpoch int64  `json:"microsecnods_since_unix_epoch"`
}

// ByTimeStamp sort.Interface interface implementation to sort events by timestamp.
type ByTimeStamp []LoginEventRecorderData

func (e ByTimeStamp) Len() int { return len(e) }
func (e ByTimeStamp) Less(i, j int) bool {
	return e[i].MicrosecnodsSinceUnixEpoch < e[j].MicrosecnodsSinceUnixEpoch
}
func (e ByTimeStamp) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

// LoginEventRecorder is helper to get login events data from Chrome.
// loginEventData is kept sorted by timestamp.
type LoginEventRecorder struct {
	prefix         string
	loginEventData []LoginEventRecorderData
	// FetchLoginEvents() will fail if LoginStarted event was not found and
	// failIfNoLoginStartedEvent flag is true.
	failIfNoLoginStartedEvent bool
}

// Prepare notifies Chrome to store a copy of login events for later retrieval.
func (t *LoginEventRecorder) Prepare(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.startLoginEventRecorderDataCollection)`); err != nil {
		return errors.Wrap(err, "failed to start data collection")
	}
	return nil
}

// FetchLoginEvents fetches and sorts all LoginEventRecorder data.
func (t *LoginEventRecorder) FetchLoginEvents(ctx context.Context, tconn *chrome.TestConn) error {
	var data []LoginEventRecorderData
	if err := tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.getLoginEventRecorderLoginEvents)`); err != nil {
		return errors.Wrap(err, "failed to fetch login events")
	}

	// Verify that we got the full list of events and LoginStated is in
	// list.
	loginStartedFound := false
	for _, event := range data {
		if event.Name == "LoginStarted" {
			loginStartedFound = true
			break
		}
	}
	if !loginStartedFound {
		if t.failIfNoLoginStartedEvent {
			return errors.New("'LoginStarted' event not found. Check that Chrome was started with --keep-login-events-for-testing flag")
		}
		testing.ContextLog(ctx, "WARNING: LoginStarted event not found. Check that Chrome was started with --keep-login-events-for-testing flag")
	}

	sort.Sort(ByTimeStamp(data))

	t.loginEventData = data
	return nil
}

// Record stores the collected data into pv for further processing.
func (t *LoginEventRecorder) Record(ctx context.Context, pv *perf.Values) {
	var prevTs int64
	if len(t.loginEventData) > 0 {
		prevTs = t.loginEventData[0].MicrosecnodsSinceUnixEpoch
	}

	for _, data := range t.loginEventData {
		metric := perf.Metric{
			Name:      t.prefix + data.Name,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}
		value := float64(data.MicrosecnodsSinceUnixEpoch-prevTs) / float64(1000)
		testing.ContextLog(ctx, "Got LoginEvent metric: ", metric, " with value ", value)
		prevTs = data.MicrosecnodsSinceUnixEpoch
		pv.Append(metric, value)
	}
}

// NewLoginEventRecorder creates a new instance for LoginEventRecorder.
func NewLoginEventRecorder(metricPrefix string, failIfNoLoginStartedEvent bool) *LoginEventRecorder {
	return &LoginEventRecorder{
		prefix:                    metricPrefix,
		failIfNoLoginStartedEvent: failIfNoLoginStartedEvent,
	}
}
