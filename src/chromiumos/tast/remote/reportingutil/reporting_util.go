// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reportingutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	grpc "google.golang.org/grpc"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	ts "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

// ReportingPoliciesDisabledUser is the path to the secert username for the policies disabled OU.
const ReportingPoliciesDisabledUser = "policy.reporting_policies_disabled_username"

// ReportingPoliciesDisabledPassword is the path to the secert password for the policies disabled OU.
const ReportingPoliciesDisabledPassword = "policy.reporting_policies_disabled_password"

// ReportingPoliciesEnabledUser is the path to the secert username for the policies enabled OU.
const ReportingPoliciesEnabledUser = "policy.reporting_policies_enabled_username"

// ReportingPoliciesEnabledPassword is the path to the secert password for the policies enabled OU.
const ReportingPoliciesEnabledPassword = "policy.reporting_policies_enabled_password"

// ManagedChromeCustomerIDPath is the path to the secret customer ID var for managedchrome.
const ManagedChromeCustomerIDPath = "policy.managedchrome_obfuscated_customer_id"

// EventsAPIKeyPath is the path to the secret api key var for the events API.
const EventsAPIKeyPath = "policy.events_api_key"

// DmServerURL is the URL to the autopush DM server.
const DmServerURL = "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api"

// ReportingServerURL is the URL to the autopush reporting server.
const ReportingServerURL = "https://autopush-chromereporting-pa.sandbox.googleapis.com/v1"

// VerifyEventTypeCallback is passed to the PruneEvents function. If this function returns false for an event
// then PruneEvents will not include the event in the returned list.
type VerifyEventTypeCallback func(InputEvent) bool

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
	Time          string         `json:"timestampMs"`
	InfoData      *InfoData      `json:"infoData"`
	TelemetryData *TelemetryData `json:"telemetryData"`
}

// InfoData mirrors the infoData JSON field.
type InfoData struct {
	MemoryInfo  *MemoryInfo  `json:"memoryInfo"`
	NetworkInfo *NetworkInfo `json:"networksInfo"`
	CpuInfo     *CpuInfo     `json:"cpuInfo"`
}

// TelemetryData mirrors the telemetryData JSON field.
type TelemetryData struct {
	AudioTelemetry       *AudioTelemetry       `json:"audioTelemetry"`
	NetworkTelemetry     *NetworkTelemetry     `json:"networksTelemetry"`
	PeripheralsTelemetry *PeripheralsTelemetry `json:"peripheralsTelemetry"`
}

// MemoryInfo mirrors the memoryInfo JSON field.
type MemoryInfo struct {
	TMEInfo *TMEInfo `json:"tmeInfo"`
}

// TMEInfo mirrors the TMEInfo JSON field.
type TMEInfo struct {
	MemoryEncryptionState     string `json:"encryptionState"`
	MaxKeys                   string `json:"maxKeys"`
	KeyLength                 string `json:"keyLength"`
	MemoryEncryptionAlgorithm string `json:"encryptionAlgorithm"`
}

type NetworkInfo struct {
	NetworkInterfaces []NetworkInterfaces `json:"networkInterfaces"`
}

type NetworkInterfaces struct {
	Type       string `json:"type"`
	MacAddress string `json:"macAddress"`
	DevicePath string `json:"devicePath"`
}

type CpuInfo struct {
	KeyLockerInfo *KeyLockerInfo `json:"keyLockerInfo"`
}

type KeyLockerInfo struct {
	Supported  bool `json:"supported"`
	Configured bool `json:"configured"`
}

// AudioTelemetry mirrors the audioTelemetry JSON field.
type AudioTelemetry struct {
	OutputMute       bool   `json:"outputMute"`
	InputMute        bool   `json:"inputMute"`
	OutputVolume     int32  `json:"outputVolume"`
	OutputDeviceName string `json:"outputDeviceName"`
	InputGain        int32  `json:"inputGain"`
	InputDeviceName  string `json:"inputDeviceName"`
}

// NetworkTelemetry mirrors the audioTelemetry JSON field.
type NetworkTelemetry struct {
	BandwithData *BandwithData `json:"bandwidthData"`
}

type BandwithData struct {
	DownloadSpeedKbps string `json:"downloadSpeedKbps"`
}

// PeripheralsTelemetry mirrors the peripheralsTelemetry JSON field.
type PeripheralsTelemetry struct {
	UsbTelemetry *UsbTelemetry `json:"usbTelemetry"`
}

// UsbTelemetry mirrors the usbTelemetry JSON field.
type UsbTelemetry struct {
	Vendor     string `json:"vendor"`
	Name       string `json:"name"`
	Vid        int32  `json:"vid"`
	Pid        int32  `json:"pid"`
	ClassId    int32  `json:"classId"`
	SubclassId int32  `json:"subclassId"`
}

type inputEventsResponse struct {
	Event []InputEvent `json:"event"`
}

// PruneEvents reduces the events response to only memory events after test began.
func PruneEvents(ctx context.Context, events []InputEvent, correctEventType VerifyEventTypeCallback) ([]InputEvent, error) {
	var prunedEvents []InputEvent
	for _, event := range events {
		if !correctEventType(event) {
			continue
		}
		prunedEvents = append(prunedEvents, event)
		j, err := json.Marshal(event)
		if err != nil {
			testing.ContextLog(ctx, "Failed to marshall event: ", err)
			return []InputEvent{}, errors.Wrap(err, "failed to marshal event")
		}
		testing.ContextLog(ctx, "Found a valid event ", string(j))
	}

	return prunedEvents, nil
}

// LookupEvents Call the Reporting API Server's ChromeReportingDebugService.LookupEvents
// endpoint to get a list of events received by the server from this user.
func LookupEvents(ctx context.Context, reportingServerURL, obfuscatedCustomerID, clientID string, apiKey, destination string, testStartTime int64) ([]InputEvent, error) {
	reqPath := fmt.Sprintf("%v/test/events?key=%v&obfuscatedCustomerId=%v&deviceId=%v&destination=%v&readDataAfterSec=%v", reportingServerURL, apiKey, obfuscatedCustomerID, clientID, destination, testStartTime)
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

// Deprovision deprovisions the DUT. This should be used after the test is over.
func Deprovision(ctx context.Context, cc grpc.ClientConnInterface, serviceAccountVar []byte, customerID string) error {
	tapeClient, err := tape.NewClient(ctx, serviceAccountVar)
	if err != nil {
		return errors.Wrap(err, "failed to create tape client")
	}

	tapeService := ts.NewServiceClient(cc)
	// Get the device id of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: "C" + customerID})
	if err != nil {
		return errors.Wrap(err, "failed to get the deviceID")
	}

	if err = tapeClient.Deprovision(ctx, res.DeviceID, customerID); err != nil {
		return errors.Wrap(err, "failed to deprovision device")
	}
	return nil
}
