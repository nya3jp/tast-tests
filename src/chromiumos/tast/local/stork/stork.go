// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package stork contains utilities for communicating with the Stork API, which creates test
// eSIM profiles.
package stork

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	// CleanupProfileTime is the time which should be allocated for running a CleanupProfileFunc.
	CleanupProfileTime = 1 * time.Minute
)

// Constants for data sent to/from Stork.
const (
	// Used in POST request.
	gtsTestProfileListKey            = "gtsTestProfileList"
	confirmationCodeValue            = ""
	maxConfirmationCodeAttemptsValue = 1
	maxDownloadAttemptsValue         = 5
	profileStatusValue               = "RELEASED"
	profileClassValue                = "OPERATIONAL"
	serviceProviderNameValue         = "CarrierConfirmationCode"
	generateSmdsEventValue           = true

	// Returned in Stork response.
	sessionIDKey  = "sessionId"
	matchingIDKey = "matchingId"
)

// curl command constants.
const (
	curlCommandName    = "curl"
	cacertArgName      = "--cacert"
	cacertArgValue     = "/usr/share/hermes-ca-certificates/test/gsma-ci.pem"
	hArgName           = "-H"
	hArgValue          = "Content-Type:application/json"
	xArgName           = "-X"
	xArgValue          = "POST"
	dataArgName        = "--data"
	startGtsSessionURL = "https://prod.smdp-plus.rsp.goog/gts/startGtsSession"
)

// Stork URLs.
const (
	// URL for discarding a Stork profile, which needs to be provided a sessionId parameter.
	endGtsSessionURLPrefix = "https://prod.smdp-plus.rsp.goog/gts/endGtsSession?sessionId="

	// Prefix for the
	activationCodePrefix = "1$prod.smdp-plus.rsp.goog$"
)

// ActivationCode to be used to install an eSIM profile.
type ActivationCode string

// CleanupProfileFunc alerts Stork that the profile has been used.
type CleanupProfileFunc func(ctx context.Context) error

// ProfileListData represents the JSON structure of profile list metadata sent to Stork.
type ProfileListData struct {
	Eid                         string `json:"eid"`
	ConfirmationCode            string `json:"confirmationCode"`
	MaxConfirmationCodeAttempts int    `json:"maxConfirmationCodeAttempts"`
	MaxDownloadAttempts         int    `json:"maxDownloadAttempts"`
	ProfileStatus               string `json:"profileStatus"`
	ProfileClass                string `json:"profileClass"`
	ServiceProviderName         string `json:"serviceProviderName"`
	GenerateSmdsEvent           bool   `json:"generateSmdsEvent"`
	ProfilePolicyRules          []int  `json:"profilePolicyRules"`
}

// RequestData represents the JSON structure of an eSIM profile request sent to Stork.
type RequestData struct {
	GtsTestProfileList []ProfileListData `json:"gtsTestProfileList"`
	Eid                string            `json:"eid"`
}

func generateStorkRequestData(eid string) (string, error) {
	profileListData := &ProfileListData{
		Eid:                         eid,
		ConfirmationCode:            confirmationCodeValue,
		MaxConfirmationCodeAttempts: maxConfirmationCodeAttemptsValue,
		MaxDownloadAttempts:         maxDownloadAttemptsValue,
		ProfileStatus:               profileStatusValue,
		ProfileClass:                profileClassValue,
		ServiceProviderName:         serviceProviderNameValue,
		GenerateSmdsEvent:           generateSmdsEventValue,
		ProfilePolicyRules:          []int{},
	}

	storkRequestData := &RequestData{
		GtsTestProfileList: []ProfileListData{*profileListData},
		Eid:                eid,
	}

	jsonBytes, err := json.Marshal(storkRequestData)
	if err != nil {
		return "", errors.Wrap(err, "JSON encoding failed")
	}

	// Stork expects JSON with single quotes.
	return strings.Replace(string(jsonBytes), "\"", "'", -1), nil
}

func getActivationCode(storkResponse map[string]json.RawMessage) (ActivationCode, error) {
	gtsTestProfileListValue, ok := storkResponse[gtsTestProfileListKey]
	if !ok {
		return ActivationCode(""), errors.Errorf("Stork response did not contain %v", gtsTestProfileListKey)
	}

	var gtsTestProfileList []json.RawMessage
	if err := json.Unmarshal(gtsTestProfileListValue, &gtsTestProfileList); err != nil {
		return ActivationCode(""), errors.Wrap(err, "invalid Stork gtsTestProfileList")
	}

	var profileInfo map[string]interface{}
	if err := json.Unmarshal(gtsTestProfileList[0], &profileInfo); err != nil {
		return ActivationCode(""), errors.Wrap(err, "Stork gtsTestProfile response was invalid")
	}

	matchingID, ok := profileInfo[matchingIDKey].(string)
	if !ok {
		return ActivationCode(""), errors.New("Stork matchingId was missing")
	}

	return ActivationCode(activationCodePrefix + matchingID), nil
}

func getSessionID(storkResponse map[string]json.RawMessage) (string, error) {
	sessionIDKeyValue, ok := storkResponse[sessionIDKey]
	if !ok {
		return "", errors.Errorf("Stork response did not contain %v", sessionIDKey)
	}

	var sessionID string
	if err := json.Unmarshal(sessionIDKeyValue, &sessionID); err != nil {
		return "", errors.Wrap(err, "invalid Stork sessionID")
	}
	return sessionID, nil
}

// FetchStorkProfile fetches a test eSIM profile from Stork.
func FetchStorkProfile(ctx context.Context, eid string) (ActivationCode, CleanupProfileFunc, error) {
	data, err := generateStorkRequestData(eid)
	if err != nil {
		return ActivationCode(""), nil, err
	}

	command := testexec.CommandContext(ctx, curlCommandName,
		cacertArgName, cacertArgValue,
		hArgName, hArgValue,
		xArgName, xArgValue,
		dataArgName, data,
		startGtsSessionURL)

	output, err := command.Output()
	if err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "failed sending Stork request")
	}

	var jsonOutput map[string]json.RawMessage
	if err := json.Unmarshal(output, &jsonOutput); err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "Stork response was invalid")
	}

	activationCode, err := getActivationCode(jsonOutput)
	if err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "could not find an activation code")
	}

	sessionID, err := getSessionID(jsonOutput)
	if err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "could not find session ID")
	}

	cleanpProfile := CleanupProfileFunc(func(ctx context.Context) error {
		command := testexec.CommandContext(ctx, curlCommandName,
			cacertArgName, cacertArgValue,
			endGtsSessionURLPrefix+sessionID)

		err := command.Run()
		if err != nil {
			return errors.Wrap(err, "failed Stork cleanup request")
		}

		return nil
	})

	return activationCode, cleanpProfile, nil
}
