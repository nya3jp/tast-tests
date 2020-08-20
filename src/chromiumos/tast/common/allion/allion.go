// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package allion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

type allionResult struct {
	ResultCode string
	ResultTxt  string
}

// RestAPI is a utility for accessing allion api.
type RestAPI struct {
	allionSvrURL string
	ctx          context.Context
}

// NewRestAPI creates a RestAPI object.
func NewRestAPI(ctx context.Context, allionURL string) *RestAPI {
	a := &RestAPI{allionURL, ctx}
	return a
}

// invokeAllion invokes Allion.
func (a *RestAPI) invokeAllion(url string, result *allionResult) error {
	apiURL := a.allionSvrURL + url
	testing.ContextLog(a.ctx, "Invoking allion apiURL: ", apiURL)
	body, statusCode, err := httputil.HTTPGet(apiURL, 30*time.Second)

	if err != nil {
		testing.ContextLog(a.ctx, "Failed to invoke allion api: ", err)
		return mtbferrors.New(mtbferrors.APIInvoke, err, apiURL)
	}

	testing.ContextLogf(a.ctx, "statusCode: %v, body: %v", statusCode, string(body))
	err = json.Unmarshal(body, result)

	if err != nil {
		testing.ContextLog(a.ctx, "Failed to unmarshall allion api result: ", err)
		return mtbferrors.New(mtbferrors.APIInvoke, err, apiURL)
	}

	return nil
}

// WifiStrManual sets the WiFi signal strength manually.
func (a *RestAPI) WifiStrManual(attnID, strength string) error {
	var result allionResult
	// apiURL := fmt.Sprintf("/wifimanual?ssid=%v&str=%v", ssid, strength)
	apiURL := fmt.Sprintf("/wifimanual?deviceID=%v&value=%v", attnID, strength)

	if err := a.invokeAllion(apiURL, &result); err != nil {
		return mtbferrors.New(mtbferrors.APIWiFiChgStrength, err, attnID, strength, result.ResultCode, result.ResultTxt)
	}

	if result.ResultCode != "0000" {
		return mtbferrors.New(mtbferrors.APIWiFiChgStrength, nil, attnID, strength, result.ResultCode, result.ResultTxt)
	}

	return nil
}

// WifiStrManualWithRetry sets the WiFi signal strength manually.
func (a *RestAPI) WifiStrManualWithRetry(attnID, strength string, retryCnt int) error {
	retry := 0
	var err error

	for retry < retryCnt {
		err = a.WifiStrManual(attnID, strength)

		if err == nil {
			testing.ContextLog(a.ctx, "WiFi strength has been changed to ", strength)
			return nil
		}

		testing.ContextLog(a.ctx, "Failed to WiFi strength: ", err)
		testing.ContextLog(a.ctx, "retry: ", retry)
		testing.Sleep(a.ctx, 1*time.Second)
		retry++
	}

	return err
}

func (a *RestAPI) ethernetUsbControl(deviceID, option string) error {
	var result allionResult
	// apiURL := fmt.Sprintf("/ethctl?deviceID=%v&option=%v", deviceID, option)
	apiURL := fmt.Sprintf("/usbctl?deviceID=%v&option=%v", deviceID, option)

	if err := a.invokeAllion(apiURL, &result); err != nil {
		return mtbferrors.New(mtbferrors.APIEthCtl, err, deviceID, option, result.ResultCode, result.ResultTxt)
	}

	if result.ResultCode != "0000" {
		return mtbferrors.New(mtbferrors.APIEthCtl, nil, deviceID, option, result.ResultCode, result.ResultTxt)
	}

	return nil
}

// EnableEthernet enables ethernet.
func (a *RestAPI) EnableEthernet(deviceID string) error {
	return a.ethernetUsbControl(deviceID, "on")
}

// EnableEthernetWithRetry enables ethernet with retries.
func (a *RestAPI) EnableEthernetWithRetry(deviceID string, retryCnt int) error {
	retry := 0
	var err error

	for retry < retryCnt {
		err = a.ethernetUsbControl(deviceID, "on")

		if err == nil {
			testing.ContextLog(a.ctx, "Ethernet is enabled")
			return nil
		}

		testing.ContextLog(a.ctx, "Failed to enable ethernet: ", err)
		testing.ContextLog(a.ctx, "retry: ", retry)
		testing.Sleep(a.ctx, 1*time.Second)
		retry++
	}

	return err
}

// DisableEthernet disables ethernet.
func (a *RestAPI) DisableEthernet(deviceID string) error {
	return a.ethernetUsbControl(deviceID, "off")
}

// EnableUsb enables USB.
func (a *RestAPI) EnableUsb(deviceID string) error {
	return a.ethernetUsbControl(deviceID, "on")
}

// DisableUsb disables USB.
func (a *RestAPI) DisableUsb(deviceID string) error {
	return a.ethernetUsbControl(deviceID, "off")
}
