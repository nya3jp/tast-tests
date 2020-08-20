// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attenuator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

// RestAPI is a utility for accessing WiFi attenuator api.
type RestAPI struct {
	attenuatorURL string
	ctx           context.Context
}

// NewRestAPI creates a RestAPI object.
func NewRestAPI(ctx context.Context, attenuatorURL string) *RestAPI {
	a := &RestAPI{attenuatorURL, ctx}
	return a
}

func (a *RestAPI) invokeAttenuator(command string) (string, error) {
	apiURL := a.attenuatorURL + command
	testing.ContextLog(a.ctx, "Invoking attenuator apiURL: ", apiURL)
	body, statusCode, err := httputil.HTTPGet(apiURL, 30*time.Second)
	responseTxt := ""

	if body != nil {
		responseTxt = string(body)
	}

	testing.ContextLogf(a.ctx, "responseTxt=%v, statusCode=%v", responseTxt, statusCode)

	if err != nil {
		testing.ContextLog(a.ctx, "Failed to invoke attenuator api: ", err)
		return "", mtbferrors.New(mtbferrors.AttenAPIInvoke, err, apiURL, statusCode, responseTxt)
	}

	if statusCode != 200 || !strings.Contains(responseTxt, "All channels set to") {
		return "", mtbferrors.New(mtbferrors.AttenAPIInvoke, err, apiURL, statusCode, responseTxt)
	}

	return responseTxt, nil
}

// SetStrength sets the WiFi signal strength manually.
func (a *RestAPI) SetStrength(strength string) error {
	command := fmt.Sprintf("?SAA%%20%s", strength)

	if response, err := a.invokeAttenuator(command); err != nil {
		return mtbferrors.New(mtbferrors.APIWiFiChgStrength, err, strength, response)
	}

	return nil
}
