// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// The Thunderbolt Quality Center (TQC) API is which exposes a REST API for automating Thunderbolt test cases.
// Package cswitch provides utilities to interact with cswitch to perform hotplug-unplug and device enumeration.
// The CSwitch contains 4 connectors. Only 1 connector at any given time is enabled. An additional
// virtual connector, referred to as '0', is used in order to simulate disconnection.
// CSwitch using FTDI controller.
package cswitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Cswitch request URLs.
const (
	// URL request for C-switch command execution.
	commandsURL = "http://%s/api/v1/commands"
	// URL request to create session.
	sessionURL = "http://%s/api/v1/sessions"
	// URL request to close session.
	closeSessionURL = "http://%s/api/v1/sessions/%s"
	// URL request to get command status.
	commandStatusURL = "http://%s/api/v1/commands/%s"
)

// performHttpRequest formats request with accept, content and application headers return http response.
func performHttpRequest(req *http.Request) (*http.Response, error) {

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-type", "application/json")

	var client http.Client
	// Request for creating a session.
	return client.Do(req)
}

// CreateSession Creates session id for controlling cswitch operations and on success it will return session id.
// If it fails it will return the respective error.
// domain holds the local host IP with port number ex: domain = "localhost:9000"
func CreateSession(ctx context.Context, domain string) (string, error) {
	testing.ContextLog(ctx, "Creating new session")
	sessionUrl := fmt.Sprintf(sessionURL, domain)
	var jsonStr = []byte(`{}`)

	// Requesting to create new session id
	req, err := http.NewRequest(http.MethodPost, sessionUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", errors.Wrapf(err, "failed to create new request for session url : %s", sessionUrl)
	}

	resp, err := performHttpRequest(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to send post request for creating a session")
	}
	defer resp.Body.Close()
	// Reading response data for session id.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("failed to read response data for session id")
	}

	if http.StatusOK != resp.StatusCode {
		return "", errors.Errorf("invalid response code received: %v", resp.StatusCode)
	}

	// Mapping response body to map data type.
	responseMap := map[string]string{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return "", err
	}

	return responseMap["session-id"], nil
}

// CloseSession closes the created session and on success it returns nil.
// If it fails it will return the respective error.
// domain holds the local host ip with port number ex: domain = "localhost:9000"
func CloseSession(ctx context.Context, sessionId string, domain string) error {
	testing.ContextLogf(ctx, "Closing Session: %s", sessionId)

	closeSessionUrl := fmt.Sprintf(closeSessionURL, domain, sessionId)

	// Creating the request for closing session
	req, err := http.NewRequest(http.MethodDelete, closeSessionUrl, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request for closing session")
	}

	resp, err := performHttpRequest(req)
	if err != nil {
		return errors.Wrap(err, "failed to send post request for creating a session")
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		return errors.Errorf("invalid response code received: %v", resp.StatusCode)
	}

	return nil
}

// execCommand Performs POST request on the given URL.
// If it fails, will return an error message .
// session_id holds session id created by CreateSession function.
// cmd_line holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func execCommand(ctx context.Context, sessionId string, cmdLine string, domain string) error {
	var cswitchResp map[string]interface{}
	jsonCmd := map[string]string{}
	jsonCmd["session-id"] = sessionId
	jsonCmd["command-line"] = fmt.Sprintf("cswitch %s", cmdLine)
	runCommandUrl := fmt.Sprintf(commandsURL, domain)
	postJson, err := json.Marshal(jsonCmd)
	if err != nil {
		return errors.Wrap(err, "unable to process JSON data ")
	}

	// Create new http post request.
	req, err := http.NewRequest(http.MethodPost, runCommandUrl, bytes.NewBuffer(postJson))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	resp, err := performHttpRequest(req)
	if err != nil {
		return errors.Wrap(err, "failed to send post request for creating a session")
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response data")
	}

	// Verify the response code to check request status.
	if http.StatusOK != resp.StatusCode {
		return errors.Errorf("invalid response code received: %v", resp.StatusCode)
	}

	if err := json.Unmarshal(body, &cswitchResp); err != nil {
		return err
	}
	// Verifies results-status "Failures","Running" and returns error.
	switch cswitchResp["result-status"].(string) {
	case "Failure":
		return errors.Errorf("%s", cswitchResp["messages"].([]interface{})[0].(map[string]interface{})["message"])
	case "Running":
		if err := waitForCommandComplete(ctx, cswitchResp["command-id"].(string), domain); err != nil {
			return errors.Wrap(err, "failed to execute command with in time")
		}
	}

	return nil
}

// commandStatus performs GET request on the given URL.
// command_id holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func commandStatus(ctx context.Context, command_id string, domain string) (string, error) {
	testing.ContextLogf(ctx, "Checking command %s status", command_id)
	responseMap := map[string]string{}
	commandStatusUrl := fmt.Sprintf(commandStatusURL, domain, command_id)

	// Creates new GET request for given URL.
	req, err := http.NewRequest(http.MethodGet, commandStatusUrl, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	resp, err := performHttpRequest(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to send post request for creating a session")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response data")
	}

	testing.ContextLogf(ctx, "Response body : %v", string(body))

	// verifies return response code and returns err if failed.
	if http.StatusOK != resp.StatusCode {
		return "", errors.Errorf("invalid response code received: %v", resp.StatusCode)
	}

	if err := json.Unmarshal(body, &responseMap); err != nil {
		return "", err
	}
	testing.ContextLogf(ctx, "response body : %v", string(body))
	return responseMap["result-status"], nil
}

// waitForCommandComplete waits till command execution completion.
// command_id holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func waitForCommandComplete(ctx context.Context, commandId string, domain string) error {
	resultStatus := "Running"

	// Waits till timeout and returns err if failed to process request with in timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status, err := commandStatus(ctx, commandId, domain)
		if err != nil {
			return testing.PollBreak(err)
		}
		if status == "Success" {
			return nil
		}
		return errors.New("still running")

	}, &testing.PollOptions{Timeout: 100 * time.Second, Interval: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "command status is %s", resultStatus)
	}
	return nil
}

// ToggleCSwitchPort enable/disable cswitch port with given parameters session id, toggle, domain ip.
// sessionId holds session id created by CreateSession function.
// toggle turns.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func ToggleCSwitchPort(ctx context.Context, sessionId, toggle string, domain string) error {
	// Executes enCSwitch command to enable/disable port on cswitch.
	return execCommand(ctx, sessionId, toggle, domain)
}

// TxSpeed returns Tx speed of the connected cable.
// port holds the TBT port id in DUT.
func TxSpeed(ctx context.Context, port string) (string, error) {
	txSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/tx_speed", port)
	// Executes txSpeedCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", txSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "txSpeedCmd execution failed")
	}
	return string(out), nil
}

// RxSpeed returns RX speed of the connected cable.
// port holds the TBT port id in DUT.
func RxSpeed(ctx context.Context, port string) (string, error) {
	rxSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/rx_speed", port)
	// Executes rxSpeedCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", rxSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "rxSpeedCmd execution failed")
	}
	return string(out), nil
}

// NvmVersion returns the NVM version of the TBT device connected to the DUT.
// port holds the TBT port id in DUT.
func NvmVersion(ctx context.Context, port string) (string, error) {
	nvmeCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/nvm_version", port)
	// Executes nvmeCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", nvmeCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "nvmeCmd execution failed")
	}
	return string(out), nil
}

// IsDeviceEnumerated validates device enumeration in DUT.
// device holds the device name of connected TBT device.
// port holds the TBT port id in DUT.
func IsDeviceEnumerated(ctx context.Context, device, port string) (bool, error) {
	deviceCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/device_name", port)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "bash", "-c", deviceCmd).Output()
		if err != nil {
			return errors.Wrapf(err, "device_name command %s failed", deviceCmd)
		}
		if strings.TrimSpace(string(out)) != device {
			return errors.New("Device enumeration failed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 3 * time.Second}); err != nil {
		return false, err
	}

	return true, nil
}

// Generation returns the generation of the TBT device connected to the DUT.
// port holds the TBT port id in DUT.
func Generation(ctx context.Context, port string) (string, error) {
	generationCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/generation", port)
	out, err := testexec.CommandContext(ctx, "bash", "-c", generationCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "generationCmd execution failed")
	}
	return string(out), nil
}
