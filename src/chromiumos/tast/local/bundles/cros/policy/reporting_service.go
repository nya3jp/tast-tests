// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const heartbeatEventDestination = "HEARTBEAT_EVENTS"

func init() {
	// This is currently being used by the remote gaia_reporting test.
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterReportingServiceServer(srv, &ReportingService{s: s})
		},
	})
}

// ReportingService implements tast.cros.policy.ReportingService.
type ReportingService struct { // NOLINT
	s *testing.ServiceState
}

type inputEvent struct {
	APIEvent *struct {
		ReportingRecordEvent *struct {
			Destination string `json:"destination"`
			Time        string `json:"timestampUs"`
		} `json:"reportingRecordEvent"`
	} `json:"apiEvent"`
	ObfuscatedCustomerID string `json:"obfuscatedCustomerID"`
	ObfuscatedGaiaID     string `json:"obfuscatedGaiaID"`
	ClientID             string `json:"clientId"`
}

// getDeviceVirtualID Returns the device's virtual ID, which is used to
// identify the enrolled device when communicating with enterprise servers.
// TODO: see if it's possible to extract the device virtual ID directly
// from the browser process in the AutotestPrivate extension.
func getDeviceVirtualID(ctx context.Context, cr *chrome.Chrome) (string, error) {
	conn, err := cr.NewConn(ctx, "chrome://policy")
	if err != nil {
		return "", errors.Wrap(err, "failed to navigate to chrome://policy page")
	}

	defer conn.Close()

	var deviceVirtualID string
	xpath := `//fieldset[./legend[./text()='Device policies']]/div/div[@class='client-id']/text()`
	evalExpr := fmt.Sprintf(`document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue.textContent`, xpath)
	if err := conn.Eval(ctx, evalExpr, &deviceVirtualID); err != nil {
		return "", errors.Wrap(err, evalExpr)
	}
	return strings.TrimSpace(deviceVirtualID), nil
}

// lookupEvents Call the Reporting API Server's ChromeReportingDebugService.LookupEvents
// endpoint to get a list of events received by the server from this user.
func lookupEvents(ctx context.Context, reportingServerURL, userEmail, obfuscatedCustomerID, apiKey string) ([]inputEvent, error) {
	type lookupEventsResponse struct {
		Event []inputEvent `json:"event"`
	}

	reqPath := fmt.Sprintf("%v/test/events?key=%v&obfuscatedCustomerId=%v", reportingServerURL, apiKey, obfuscatedCustomerID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to craft event query request to the Reporting Server")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to issue debug query request to the Reporting Server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.Errorf("reporting server encountered an error with the event query %s %v %v", reqPath, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the response body")
	}
	var resData lookupEventsResponse
	if err := json.Unmarshal(resBody, &resData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return resData.Event, nil
}

// validateEventReception Given a list of events received by the Reporting API Server, validate whether the list contains events sent by this test.
func validateEventReception(events []inputEvent, deviceVirtualID, eventDestination string, testStartTime time.Time, containsEvent *bool) error {
	*containsEvent = false
	for _, event := range events {
		// Filter events by the device virtual ID, which uniquely identifies
		// this test device, and by the event destination, which identifies the
		// event type.
		if event.ClientID == deviceVirtualID && event.APIEvent.ReportingRecordEvent.Destination == eventDestination {
			// Parse the timestamp and check if the server received the event
			// after test started.
			micros, err := strconv.ParseInt(event.APIEvent.ReportingRecordEvent.Time, 10, 64)
			if err != nil {
				return errors.Wrap(err, "failed to parse int64 Spanner timestamp from event")
			}
			enqueueTime := time.Unix(micros/(int64(time.Second)/int64(time.Microsecond)), 0)
			if enqueueTime.After(testStartTime) {
				// Found an event sent by this test device after the test start time.
				// This proves that the server received events from this device.
				*containsEvent = true
				return nil
			}
		}
	}
	return nil
}

// GAIAEnrollUsingChromeAndCollectReporting enrolls the device using dmserver. Specified user is logged in after this function completes.
func (c *ReportingService) GAIAEnrollUsingChromeAndCollectReporting(ctx context.Context, req *ppb.GAIAEnrollUsingChromeAndCollectReportingRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with username: %s, dmserver: %s", string(req.Username), string(req.DmserverURL))

	testStartTime := time.Now()

	cr, err := chrome.New(
		ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.DMSPolicy(req.DmserverURL),
		chrome.EnableFeatures("EncryptedReportingPipeline", "EncryptedReportingHeartbeatEvent"),
		chrome.EncryptedReportingAddr(fmt.Sprintf("%v/record", req.ReportingserverURL)),
		chrome.ExtraArgs("--login-manager"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	tconn, err := cr.TestAPIConn(ctx)

	// Check that the cloud reporting policy is enabled.
	testing.ContextLog(ctx, "Verify that Cloud Reporting is enabled")
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.CloudReportingEnabled{Val: true}}); err != nil {
		errors.Wrap(err, "failed to verify Real-Time Reporting policy")
	}

	deviceVirtualID, err := getDeviceVirtualID(ctx, cr)
	if err != nil {
		errors.Wrap(err, "cannot get Device virtual ID")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := lookupEvents(ctx, req.ReportingserverURL, req.Username, req.ObfuscatedCustomerID, req.DebugServiceAPIKey)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to look up events"))
		}

		receivedNewEvents := false
		if err := validateEventReception(events, deviceVirtualID, heartbeatEventDestination, testStartTime, &receivedNewEvents); err != nil {
			errors.Wrap(err, "failed to validate event reception")
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		errors.Wrap(err, "the Reporting Api Server did not receive any events")
	}

	defer cr.Close(ctx)
	return &empty.Empty{}, nil
}
