// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

	"chromiumos/tast/errors"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

const (
	// Success response code for URL request.
	successCode = 200
	// Application header in URL request.
	applicationHeader = "application/json"
	// Accept header in URL request.
	acceptHeader      = "Accept"
	// Content header in URL request.
	contentHeader     = "Content-type"
)

// Cswitch request URLs.
var (
	// URL request for C-switch command execution.
	commandsURL      = "http://%s/api/v1/commands"
	// URL request to create session.
	sessionURL       = "http://%s/api/v1/sessions"
    // URL request to close session.
	closeSessionURL  = "http://%s/api/v1/sessions/%s"
	// URL request to get command status.
	commandStatusURL = "http://%s/api/v1/commands/%s"
)

// CreateSession func Create session id for controlling cswitch operations and on success it will returns session id.
// If it fails it will return the respective error.
// domain it holds the local host ip with port number ex: domain = "localhost:9000"
func CreateSession(ctx context.Context, domain string) (string, error) {
	testing.ContextLog(ctx, "Creating new session")
	var client http.Client
	sessionUrl := fmt.Sprintf(sessionURL, domain)
	var jsonStr = []byte(`{}`)

   // Requesting to create new session id
	req, err := http.NewRequest("POST", sessionUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return  "", errors.Wrapf(err, "Failed to create new request for session url : %s", sessionUrl)
	}

	req.Header.Add(acceptHeader, applicationHeader)
	req.Header.Add(contentHeader, applicationHeader)

   // Request for creating a session.
	resp, err := client.Do(req)
	if err != nil {
		return "",errors.Wrap(err, "Failed to send post request for creating a session")
	}
	defer resp.Body.Close()
   // Reading response data for session id.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Failed to read response data for session id")
	}
   // verify the response code to check request status.
	if resp.StatusCode != successCode {
		response_err := fmt.Sprintf("Invalid response code received: %v", resp.StatusCode)
		return "", errors.New(response_err)
	}

	// Mapping response body to map data type.
	responseMap := map[string]string{}
	if err := json.Unmarshal([]byte(string(body)), &responseMap); err != nil {
		return "", err
	}

	testing.ContextLogf(ctx, "Session ID: %s", responseMap["session-id"])

	return  responseMap["session-id"], nil
}

// CloseSession func Closing the created session id and on success it will returns nil.
// If it fails it will return the respective error.
// domain it holds the local host ip with port number ex: domain = "localhost:9000"
func CloseSession(ctx context.Context, session_id string, domain string) error {
	testing.ContextLogf(ctx, "Closing Session: %s", session_id)
	var client http.Client
	closeSessionUrl := fmt.Sprintf(closeSessionURL, domain, session_id)

    // Creating the request for closing session
	req, err := http.NewRequest("DELETE", closeSessionUrl, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create request for closing session")
	}

	req.Header.Add(acceptHeader, applicationHeader)
	req.Header.Add(contentHeader, applicationHeader)

    // Requesting to delete a session.
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send delete request for session")
	}
	defer resp.Body.Close()
    // verify the response code to check request status.
	if resp.StatusCode != successCode {
		response_err := fmt.Sprintf("Invalid response code received: %v", resp.StatusCode)
		return errors.New(response_err)
	}
	return nil
}

// RunCommand func Performs POST request on the given URL.
// It will return  200 ok response along with response body.
// if it fails, will return an error message .
// session_id holds session id created by CreateSession function.
// cmd_line holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func RunCommand(ctx context.Context, session_id string, cmd_line string, domain string) (map[string]interface{}, error) {
	var cswitchResp map[string]interface{}
	jsonCmd := map[string]string{}
	jsonCmd["session-id"] = session_id
	jsonCmd["command-line"] = cmd_line
	var client http.Client
	runCommandUrl := fmt.Sprintf(commandsURL, domain)
	postJson, err := json.Marshal(jsonCmd)
	if err != nil {
		return nil, err 
	}

	// Creates new http post request.
	req, err := http.NewRequest("POST", runCommandUrl, bytes.NewBuffer(postJson))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create request")
	}

	req.Header.Add(acceptHeader, applicationHeader)
	req.Header.Add(contentHeader, applicationHeader)

	// It will process newly created http request.
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send post request")
	}
	defer resp.Body.Close()

	// Reads the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Failed to read response data")
	}

	// verify the response code to check request status.
	if resp.StatusCode != successCode {
		response_err := fmt.Sprintf("Invalid response code received: %v", resp.StatusCode)
		return nil, errors.New(response_err)
	}

	if err := json.Unmarshal([]byte(string(body)), &cswitchResp); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "response map: %v", cswitchResp)

	return cswitchResp, nil 
}

// ExecCommand func Executes function  RunCommand and validates the return response.
// session_id holds session id created by CreateSession function.
// cmd_line holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func ExecCommand(ctx context.Context, session_id string, cmd_line string, domain string) error {
	// Executes cmd_line if any error returns err.
	resp, err := RunCommand(ctx, session_id, cmd_line, domain)
	if err != nil {
		return err
	}

	// Waits till http request completion till time out.
	if resp["result-status"].(string) == "Running" {
		if err := waitForCommandComplete(ctx, resp["command-id"].(string), domain); err != nil {
			return err
		}
	}
	testing.ContextLogf(ctx, "%s PASSED", cmd_line)
	return nil
}

// CommandStatus Performs GET request on the given URL.
// command_id holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func CommandStatus(ctx context.Context, command_id string, domain string) (map[string]string, error) {
	testing.ContextLogf(ctx, "Checking command %s status", command_id)
	responseMap := map[string]string{}
	var client http.Client

	commandStatusUrl := fmt.Sprintf(commandStatusURL, domain, command_id)

	// Creates new GET request for given URL.
	req, err := http.NewRequest("GET", commandStatusUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create request")
	}

	req.Header.Add(acceptHeader, applicationHeader)
	req.Header.Add(contentHeader, applicationHeader)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send GET request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Failed to read response data")
	}

	testing.ContextLogf(ctx, "response body : %v", string(body))

	// verifies return response code and returns err if failed.
	if resp.StatusCode != successCode {
		response_err := fmt.Sprintf("Invalid response code received: %v", resp.StatusCode)
		return nil, errors.New(response_err)
	}

	if err := json.Unmarshal([]byte(string(body)), &responseMap); err != nil {
		return nil, err
	}

	return responseMap, nil
}

// CommandStatus Performs GET request on the given URL.
// command_id holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func waitForCommandComplete(ctx context.Context, command_id string, domain string) error {
	result_status := "Running"

	// Waits till timeout and returns err if failed to process request with in timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if result_status != "Success" {
			resp, err := CommandStatus(ctx, command_id, domain)
			if err != nil {
				return err
			}

			result_status = resp["result-status"]
			return errors.New("still running")

		}
		return nil
	}, &testing.PollOptions{Timeout: 300 * time.Second, Interval: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "result status is %s", result_status)
	}
	return nil
}

// EnableCSwitchPort func Enables cswitch port with given parameters session id, cswitch port, domain ip.
// session_id holds session id created by CreateSession function.
// cmd_line holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func EnableCSwitchPort(ctx context.Context, session_id, cswitch string, domain string) error {
	enCSwitch := fmt.Sprintf("cswitch %s", cswitch)
	testing.ContextLog(ctx, "Enabling C-Switch port")

	// Executes enCSwitch command to enable port on cswitch.
	if err := ExecCommand(ctx, session_id, enCSwitch, domain); err != nil {
		return err
	}
	return nil
}

// DisableCSwitchPort func Disable cswitch port with given parameters session id, domain ip.
// session_id holds session id created by CreateSession function.
// cmd_line holds command need to executed.
// domain it holds the local host ip with port number ex: domain = "localhost:9000".
func DisableCSwitchPort(ctx context.Context, session_id string, domain string) error {
	testing.ContextLog(ctx, "Disabling C-Switch port")

	// Executes cswitch 0 command to enable port on cswitch.
	if err := ExecCommand(ctx, session_id, "cswitch 0", domain); err != nil {
		return err
	}
	return nil
}

// TxRxSpeed func Validates TX, RX speed of the connected device in DUT.
// port holds the tbt port id in dut.
// txSpeed, rxSpeed are the expected cable speed connected to dut.
func TxRxSpeed(ctx context.Context, port, txSpeed, rxSpeed string) error {
	txSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/tx_speed", port)

	// Executes txSpeedCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", txSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "tx_speed command %s failed", txSpeedCmd)
	}

	// verifies expected tx speed.
	txspeedResult := strings.TrimSpace(string(out))
	if txspeedResult != txSpeed {
		return errors.Errorf("%s is not expected tx speed", txSpeed)
	}

	testing.ContextLogf(ctx, "%s is the expected tx speed", txSpeed)

	// Executes rxSpeedCmd and returns err on command execution failure.
	rxSpeedCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/rx_speed", port)
	rxOut, err := testexec.CommandContext(ctx, "bash", "-c", rxSpeedCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "rx_speed command failed")
	}

	// verifies expected rx speed.
	rxSpeedResult := strings.TrimSpace(string(rxOut))
	if rxSpeedResult != rxSpeed {
		return errors.Errorf("%s is not expected rx speed", rxSpeed)
	}

	testing.ContextLogf(ctx, "%s is the expected rx speed", rxSpeed)

	return nil
}

// NvmeVersion func Validate NVME version of the connected device in DUT.
// port holds the tbt port id in dut.
// tbtNvmeVersion is the  expected nvme version of the connected tbt device.
func NvmeVersion(ctx context.Context, port, tbtNvmeVersion string) error {
	nvmeCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/nvm_version", port)

	// Executes nvmeCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", nvmeCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "nvme_version command %s failed", nvmeCmd)
	}

	// Verifies expected NvmeVersion.
	if strings.TrimSpace(string(out)) != tbtNvmeVersion {
		return errors.Errorf("%s is not expected nvme version", tbtNvmeVersion)
	}

	testing.ContextLogf(ctx, "%s is the expected nvme version", tbtNvmeVersion)
	
	return nil
}

// IsDeviceEnumerated func Validates device enumeration in DUT.
// device holds the device name of connected tbt device.
// port holds the tbt port id in dut.
func IsDeviceEnumerated(ctx context.Context, device, port string) error {
	deviceCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/device_name", port)

	// Executes deviceCmd and returns err on command execution failure.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "bash", "-c", deviceCmd).Output()
		if err != nil {
			return errors.Wrapf(err, "device_name command %s failed", deviceCmd)
		}

		// Verifies expected device name.
		if strings.TrimSpace(string(out)) != device {
			return errors.New("Device enumeration failed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 3 * time.Second}); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "%s is the expected device name", device)

	return nil
}

// Generation func Validate generation of the connected device in DUT.
// port holds the tbt port id in dut.
// tbtGeneration holds the device name of connected tbt device.
func Generation(ctx context.Context, port, tbtGeneration string) error {
	generationCmd := fmt.Sprintf("cat /sys/bus/thunderbolt/devices/%s/generation", port)

	// Executes generationCmd and returns err on command execution failure.
	out, err := testexec.CommandContext(ctx, "bash", "-c", generationCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "Generation command %s failed", generationCmd)
	}

	// Verifies expected device Generation.
	if strings.TrimSpace(string(out)) != tbtGeneration {
		return errors.Errorf("%s is not expected generation", tbtGeneration)
	}

	testing.ContextLogf(ctx, "%s is the expected generation", tbtGeneration)

	return nil
}