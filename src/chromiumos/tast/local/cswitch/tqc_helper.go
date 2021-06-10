// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cswitch provides utilities to interact with cswitch to perform hotplug-unplug and device enumeration.
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

const (
	// Success response code for URL request.
	successCode = 200
	// Application header in URL request.
	applicationHeader = "application/json"
	// Accept header in URL request.
	acceptHeader = "Accept"
	// Content header in URL request.
	contentHeader = "Content-type"
)

// Cswitch request URLs.
var (
	// URL request for C-switch command execution.
	commandsURL = "http://%s/api/v1/commands"
	// URL request to create session.
	sessionURL = "http://%s/api/v1/sessions"
	// URL request to close session.
	closeSessionURL = "http://%s/api/v1/sessions/%s"
	// URL request to get command status.
	commandStatusURL = "http://%s/api/v1/commands/%s"
)

// reqHeaderAdd formats request with accept, content and application headers.
func reqHeaderAdd(req *http.Request) {

	req.Header.Add(acceptHeader, applicationHeader)
	req.Header.Add(contentHeader, applicationHeader)

}

// CreateSession  Creates session id for controlling cswitch operations and on success it will returns session id.
// If it fails it will return the respective error.
// domain holds the local host ip with port number ex: domain = "localhost:9000"
func CreateSession(ctx context.Context, domain string) (string, error) {
	testing.ContextLog(ctx, "Creating new session")
	sessionUrl := fmt.Sprintf(sessionURL, domain)
	var jsonStr = []byte(`{}`)

	// Requesting to create new session id
	req, err := http.NewRequest("POST", sessionUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create new request for session url : %s", sessionUrl)
	}

	reqHeaderAdd(req)

	var client http.Client
	// Request for creating a session.
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to send post request for creating a session")
	}
	defer resp.Body.Close()
	// Reading response data for session id.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("failed to read response data for session id")
	}

	if resp.StatusCode != successCode {
		return "", errors.Errorf("Invalid response code received: %v", resp.StatusCode)
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
	req, err := http.NewRequest("DELETE", closeSessionUrl, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request for closing session")
	}

	reqHeaderAdd(req)

	var client http.Client
	// Requesting to delete a session.
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send delete request for session")
	}
	defer resp.Body.Close()

	if resp.StatusCode != successCode {
		return errors.Errorf("Invalid response code received: %v", resp.StatusCode)
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
	jsonCmd["command-line"] = cmdLine
	runCommandUrl := fmt.Sprintf(commandsURL, domain)
	postJson, err := json.Marshal(jsonCmd)
	if err != nil {
		return err
	}

	// Creates new http post request.
	req, err := http.NewRequest("POST", runCommandUrl, bytes.NewBuffer(postJson))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	reqHeaderAdd(req)

	var client http.Client
	// It will process newly created http request.
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send post request")
	}
	defer resp.Body.Close()

	// Reads the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response data")
	}

	// verify the response code to check request status.
	if resp.StatusCode != successCode {
		return errors.Errorf("Invalid response code received: %v", resp.StatusCode)
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
			return err
		}
	}

	return nil
}

// commandStatus performs GET request on the given URL.
// command_id holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func commandStatus(ctx context.Context, command_id string, domain string) (map[string]string, error) {
	testing.ContextLogf(ctx, "Checking command %s status", command_id)
	responseMap := map[string]string{}
	commandStatusUrl := fmt.Sprintf(commandStatusURL, domain, command_id)

	// Creates new GET request for given URL.
	req, err := http.NewRequest("GET", commandStatusUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	reqHeaderAdd(req)

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send GET request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response data")
	}

	testing.ContextLogf(ctx, "response body : %v", string(body))

	// verifies return response code and returns err if failed.
	if resp.StatusCode != successCode {
		return nil, errors.Errorf("invalid response code received: %v", resp.StatusCode)
	}

	if err := json.Unmarshal(body, &responseMap); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "response body : %v", string(body))
	return responseMap, nil
}

// waitForCommandComplete waits till command execution completion.
// command_id holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func waitForCommandComplete(ctx context.Context, command_id string, domain string) error {
	result_status := "Running"

	// Waits till timeout and returns err if failed to process request with in timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		resp, err := commandStatus(ctx, command_id, domain)
		if err != nil {
			return testing.PollBreak(err)
		}
		result_status = resp["result-status"]
		if result_status == "Success" {
			return nil
		}
		return errors.New("still running")

	}, &testing.PollOptions{Timeout: 300 * time.Second, Interval: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "result status is %s", result_status)
	}
	return nil
}

// ToggleCSwitchPort enable/disable cswitch port with given parameters session id, cswitch port, domain ip.
// sessionId holds session id created by CreateSession function.
// cmdLine holds command need to executed.
// domain holds the local host ip with port number ex: domain = "localhost:9000".
func ToggleCSwitchPort(ctx context.Context, sessionId, cswitch string, domain string) error {
	// Executes enCSwitch command to enable/disable port on cswitch.
	return execCommand(ctx, sessionId, cswitch, domain)
}

// TxSpeed returns Tx speed of the connected cable.
// port holds the tbt port id in dut.
func TxSpeed(ctx context.Context, port string) (string, error) {
	txSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/tx_speed", port)
	// Executes txSpeedCmd and returns err on command execution failure.
	Out, err := testexec.CommandContext(ctx, "bash", "-c", txSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "txSpeedCmd execution failed")
	}
	return string(Out), nil
}

// RxSpeed returns RX speed of the connected cable.
// port holds the tbt port id in dut.
func RxSpeed(ctx context.Context, port string) (string, error) {
	rxSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/rx_speed", port)
	// Executes rxSpeedCmd and returns err on command execution failure.
	Out, err := testexec.CommandContext(ctx, "bash", "-c", rxSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "rxSpeedCmd execution failed")
	}
	return string(Out), nil
}

// NvmVersion  returns the NVM version of the connected device in DUT.
// port holds the tbt port id in dut.
func NvmVersion(ctx context.Context, port string) (string, error) {
	nvmeCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/nvm_version", port)
	// Executes nvmeCmd and returns err on command execution failure.
	Out, err := testexec.CommandContext(ctx, "bash", "-c", nvmeCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "nvmeCmd execution failed")
	}
	return string(Out), nil
}

// IsDeviceEnumerated  validates device enumeration in DUT.
// device holds the device name of connected tbt device.
// port holds the tbt port id in dut.
func IsDeviceEnumerated(ctx context.Context, device, port string) (bool, error) {
	deviceCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/device_name", port)
	// Executes deviceCmd and returns err on command execution failure.
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

// Generation returns the generation of the connected device in DUT.
// port holds the tbt port id in dut.
func Generation(ctx context.Context, port string) (string, error) {
	generationCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/generation", port)
	// Executes generationCmd and returns err on command execution failure.
	Out, err := testexec.CommandContext(ctx, "bash", "-c", generationCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "generationCmd execution failed")
	}
	return string(Out), nil
}
