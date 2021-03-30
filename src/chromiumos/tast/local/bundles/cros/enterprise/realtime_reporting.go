// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type testParams struct {
	username        string // username for Chrome loging
	password        string // password to login
	dmserver        string // device management server url
	reportingserver string // reporting api server url
}

type inputEvent struct {
	event *struct {
		time   string `json:"created_time"`
		device *struct {
			clientID string `json:"client_id"`
		} `json:"event_device"`
	} `json:"event"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RealtimeReporting,
		Desc:         "Check that Chrome can correctly report real-time events to the Reporting Server",
		Contacts:     []string{"uwyiming@google.com" /*, "cros-reporting-team@google.com"*/},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testParams{
					username:        "enterprise.RealtimeReporting.user_name",
					password:        "enterprise.RealtimeReporting.password",
					dmserver:        "crosman-alpha.sandbox",
					reportingserver: "autopush-chromereporting-pa.sandbox.googleapis.com",
				},
			},
		},
		Vars: []string{
			"enterprise.RealtimeReporting.user_name",
			"enterprise.RealtimeReporting.password",
			"enterprise.RealtimeReporting.lookup_events_api_key",
		},
	})
}

// lookupEvents Call the Reporting API Server's ChromeReportingDebugService.LookupEvents
// endpoint to get a list of events received by the server from this user.
func lookupEvents(ctx context.Context, reportingServerURL, userEmail, apiKey string) ([]inputEvent, error) {
	type lookupEventsRequest struct {
		userEmail string   `json:"user_email"`
		eventID   []string `json:"event_id`
	}

	type lookupEventsResponse struct {
		event []inputEvent `json:"event"`
	}

	// TODO craft request path
	reqPath := fmt.Sprintf("http://%v/", reportingServerURL)
	req, err := http.NewRequestWithContext(ctx, "POST", reqPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to craft event query request to the Reporting Server")
	}
	reqData := lookupEventsRequest{
		userEmail: userEmail,
		eventID:   []string{"TODO: add heartbeat test event ID"},
	}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBody))
	req.ContentLength = int64(len(reqBody))
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to issue debug query request to the Reporting Server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.Errorf("reporting server encountered an error with the event query %v %v", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap("failed to read the response body", err)
	}
	var resData lookupEventsResponse
	if err := json.Unmarshal(resBody, &resData); err != nil {
		return errors.Wrap("failed to unmarshal response", err)
	}
	return resData.event, nil
}

// validateEventReception Given a list of events received by the Reporting
// API Server, validate whether the list contains events sent by this test.
func validateEventReception(inputs []inputEvent, deviceVirtualID string, testStartTime Time) (bool, error) {
	for _, input := range inputs {
		// Filter events by the device virtual ID, whcih uniquely identifies
		// this test device.
		if input.event.device.clientID == deviceVirtualID {
			// Parse the timestamp and check if the server received the event
			// after test started.
			if receptionT, err := time.Parse(time.RFC3339, input.event.time); err == nil && receptionT.After(testStartTime) {
				// Found an event sent by this test device after the test start time.
				// This proves that the server received events from this device.
				return true, nil
			}
		}
	}
	return false, nil
}

func RealtimeReporting(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 10 * time.Second // time reserved for cleanup.
	)
	param := s.Param().(testParams)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver
	reportingServerURL := param.reportingserver
	debugServiceAPIKey := s.RequiredVar("enterprise.RealtimeReporting.lookup_events_api_key")

	// Log-in to Chrome
	cr, err := chrome.New(
		ctx,
		chrome.EnterpriseEnroll(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(dmServerURL),
		chrome.RTReportingServer(reportingServerURL),
		chrome.ExtraArgs("--flag=value TODO add heartbeat event flag here"),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Use a shorter context to leave time for cleanup
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)

	// Ensure chrome://policy shows the correct setting for reporting
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.CloudReportingEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to verify Real-Time Reporting policy: ", err)
	}

	// TODO: trigger test event
	// or wait for the heartbeat event to trigger

	// Call the Reporting Server's lookupEvents API to verify that
	// the server received the test event.
}
