// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package stork contains utilities for communicating with the Stork API, which creates
// test eSIM profiles.
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
	CleanupProfileTime = 5 * time.Second

	// Used for POST data provided to Stork.
	gtsTestProfileListKey            = "gtsTestProfileList"
	eidKey                           = "eid"
	eidValue                         = ""
	confirmationCodeKey              = "confirmationCode"
	confirmationCodeValue            = ""
	maxConfirmationCodeAttemptsKey   = "maxConfirmationCodeAttempts"
	maxConfirmationCodeAttemptsValue = 1
	maxDownloadAttemptsKey           = "maxDownloadAttempts"
	maxDownloadAttemptsValue         = 5
	profileStatusKey                 = "profileStatus"
	profileStatusValue               = "RELEASED"
	profileClassKey                  = "profileClass"
	profileClassValue                = "OPERATIONAL"
	serviceProviderNameKey           = "serviceProviderName"
	serviceProviderNameValue         = "CarrierConfirmationCode"
	generateSmdsEventKey             = "generateSmdsEvent"
	generateSmdsEventValue           = true
	profilePolicyRulesKey            = "profilePolicyRules"

	// curl command constants.
	curlCommandName = "curl"
	cacertArgName   = "--cacert"
	cacertArgValue  = "/usr/share/hermes-ca-certificates/test/gsma-ci.pem"
	hArgName        = "-H"
	hArgValue       = "Content-Type:application/json"
	xArgName        = "-X"
	xArgValue       = "POST"
	dataArgName     = "--data"
	url             = "https://prod.smdp-plus.rsp.goog/gts/startGtsSession"

	// Stork response constants.
	sessionIDKey  = "sessionId"
	matchingIDKey = "matchingId"

	// URL for discarding a Stork profile, which needs to be provided a sessionId parameter.
	endGtsSessionURLPrefix = "https://prod.smdp-plus.rsp.goog/gts/endGtsSession?sessionId="

	// Prefix for the
	activationCodePrefix = "1$prod.smdp-plus.rsp.goog$"
)

// ActivationCode to be used to install an eSIM profile.
type ActivationCode string

// CleanupProfileFunc alerts Stork that the profile has been used.
type CleanupProfileFunc func(ctx context.Context) error

func generateStorkRequestData() (string, error) {
	profileListData := make(map[string]interface{})
	profileListData[eidKey] = eidValue
	profileListData[confirmationCodeKey] = confirmationCodeValue
	profileListData[maxConfirmationCodeAttemptsKey] = maxConfirmationCodeAttemptsValue
	profileListData[maxDownloadAttemptsKey] = maxDownloadAttemptsValue
	profileListData[profileStatusKey] = profileStatusValue
	profileListData[profileClassKey] = profileClassValue
	profileListData[serviceProviderNameKey] = serviceProviderNameValue
	profileListData[generateSmdsEventKey] = generateSmdsEventValue
	profileListData[profilePolicyRulesKey] = []int{}

	requestData := make(map[string]interface{})
	requestData[gtsTestProfileListKey] = []map[string]interface{}{profileListData}
	requestData[eidKey] = eidValue

	jsonBytes, err := json.Marshal(requestData)
	if err != nil {
		return "", errors.Wrap(err, "JSON encoding failed")
	}

	// Stork expects JSON with single quotes.
	return strings.Replace(string(jsonBytes), "\"", "'", -1), nil
}

func getActivationCode(storkResponse map[string]json.RawMessage) (ActivationCode, error) {
	var gtsTestProfileList []json.RawMessage
	if err := json.Unmarshal(storkResponse[gtsTestProfileListKey], &gtsTestProfileList); err != nil {
		return ActivationCode(""), errors.Wrap(err, "Stork response did not contain gtsTestProfileList")
	}

	var profileInfo map[string]interface{}
	if err := json.Unmarshal(gtsTestProfileList[0], &profileInfo); err != nil {
		return ActivationCode(""), errors.Wrap(err, "Stork gtsTestProfile response is not valid JSON")
	}

	matchingID, ok := profileInfo[matchingIDKey].(string)
	if !ok {
		return ActivationCode(""), errors.New("Stork matchingId is missing")
	}

	return ActivationCode(activationCodePrefix + matchingID), nil
}

func createEndSessionFunc(ctx context.Context, storkResponse map[string]json.RawMessage) (CleanupProfileFunc, error) {
	var sessionID string
	if err := json.Unmarshal(storkResponse[sessionIDKey], &sessionID); err != nil {
		return nil, errors.Wrap(err, "Stork response did not contain sessionId")
	}

	return CleanupProfileFunc(func(ctx context.Context) error {
		command := testexec.CommandContext(ctx, curlCommandName,
			cacertArgName, cacertArgValue,
			endGtsSessionURLPrefix+sessionID)

		_, err := command.Output()
		if err != nil {
			return errors.Wrap(err, "failed Stork cleanup request")
		}

		return nil
	}), nil
}

// FetchStorkProfile fetches a test eSIM profile from Stork.
func FetchStorkProfile(ctx context.Context) (ActivationCode, CleanupProfileFunc, error) {
	data, err := generateStorkRequestData()
	if err != nil {
		return ActivationCode(""), nil, err
	}

	command := testexec.CommandContext(ctx, curlCommandName,
		cacertArgName, cacertArgValue,
		hArgName, hArgValue,
		xArgName, xArgValue,
		dataArgName, data,
		url)

	output, err := command.Output()
	if err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "failed Stork request")
	}

	var jsonOutput map[string]json.RawMessage
	if err := json.Unmarshal(output, &jsonOutput); err != nil {
		return ActivationCode(""), nil, errors.Wrap(err, "Stork response is not valid JSON")
	}

	activationCode, err := getActivationCode(jsonOutput)
	if err != nil {
		return ActivationCode(""), nil, err
	}

	endSessionFunc, err := createEndSessionFunc(ctx, jsonOutput)
	if err != nil {
		return ActivationCode(""), nil, err
	}

	return activationCode, endSessionFunc, nil
}
