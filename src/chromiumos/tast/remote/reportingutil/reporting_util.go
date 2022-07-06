// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reportingutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"chromiumos/tast/errors"
)

// ReportingPoliciesDisabledUser is the path to the secert username for the policies disabled OU.
const ReportingPoliciesDisabledUser = "policy.reporting_policies_disabled_usename"

// ReportingPoliciesDisabledPassword is the path to the secert password for the policies disabled OU.
const ReportingPoliciesDisabledPassword = "policy.reporting_policies_disabled_password"

// ManagedChromeCustomerIDPath is the path to the secret customer ID var for managedchrome.
const ManagedChromeCustomerIDPath = "policy.managedchrome_obfuscated_customer_id"

// EventsAPIKeyPath is the path to the secret api key var for the events API.
const EventsAPIKeyPath = "policy.events_api_key"

// DmServerURL is the URL to the autopush DM server.
const DmServerURL = "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api"

// ReportingServerURL is the URL to the autopush reporting server.
const ReportingServerURL = "https://autopush-chromereporting-pa.sandbox.googleapis.com/v1"

// InputEvent is the model for the response from Reporting API. Add to this
// when you want to query for new fields.
type InputEvent struct {
	APIEvent *struct {
		ReportingRecordEvent *struct {
			Destination string `json:"destination"`
			Time        string `json:"timestampUs"`
		} `json:"reportingRecordEvent"`
	} `json:"apiEvent"`
	ObfuscatedCustomerID string                `json:"obfuscatedCustomerID"`
	ObfuscatedGaiaID     string                `json:"obfuscatedGaiaID"`
	ClientID             string                `json:"clientId"`
	WrappedEncryptedData *WrappedEncryptedData `json:"wrappedEncryptedData"`
}

// WrappedEncryptedData mirrors the wrappedEncryptedData JSON field.
type WrappedEncryptedData struct {
	MetricData *MetricData `json:"metricData"`
}

// MetricData mirrors the metricData JSON field.
type MetricData struct {
	Time     string    `json:"timestampMs"`
	InfoData *InfoData `json:"infoData"`
}

// InfoData mirrors the infoData JSON field.
type InfoData struct {
	MemoryInfo *MemoryInfo `json:"memoryInfo"`
}

// MemoryInfo mirrors the memoryInfo JSON field.
type MemoryInfo struct {
	TMEInfo *TMEInfo `json:"tmeInfo"`
}

// TMEInfo mirrors the TMEInfo JSON field.
type TMEInfo struct {
	MemoryEncryptionState     string `json:"encryptionState"`
	MaxKeys                   int64  `json:"maxKeys"`
	KeyLength                 int64  `json:"keyLength"`
	MemoryEncryptionAlgorithm string `json:"encryptionAlgorithm"`
}

type inputEventsResponse struct {
	Event []InputEvent `json:"event"`
}

// LookupEvents Call the Reporting API Server's ChromeReportingDebugService.LookupEvents
// endpoint to get a list of events received by the server from this user.
func LookupEvents(ctx context.Context, reportingServerURL, obfuscatedCustomerID, apiKey, destination string) ([]InputEvent, error) {
	reqPath := fmt.Sprintf("%v/test/events?key=%v&obfuscatedCustomerId=%v&destination=%v", reportingServerURL, apiKey, obfuscatedCustomerID, destination)
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
		return nil, errors.Errorf("reporting server encountered an error with the event query %q %v %q", reqPath, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the response body")
	}
	var resData inputEventsResponse
	if err := json.Unmarshal(resBody, &resData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return resData.Event, nil
}
