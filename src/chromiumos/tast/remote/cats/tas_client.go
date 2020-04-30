// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cats

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// QueryDUTInfoReq is the body of QueryDUTInfo.
type QueryDUTInfoReq struct {
	WSClientID   string `json:"wsClientId"`
	TaskID       string `json:"taskId"`
	TaskPlan     string `json:"taskPlan"`
	TaskFilePath string `json:"taskFilePath"`
}

// DUTInfo has DUT Info.
type DUTInfo struct {
	DutOid         int         `json:"dutOid"`
	DroneOid       int         `json:"droneOid"`
	DutID          string      `json:"dutId"`
	DutIP          string      `json:"dutIp"`
	CdpPort        int         `json:"cdpPort"`
	DutStatus      string      `json:"dutStatus"`
	DeviceID       string      `json:"deviceId"`
	OemOid         int         `json:"oemOid"`
	Platform       string      `json:"platform"`
	Board          string      `json:"board"`
	CreateTime     string      `json:"createTime"`
	UpdateTime     string      `json:"updateTime"`
	PcList         interface{} `json:"pcList"`
	NodeIP         string      `json:"nodeIp"`
	NodePort       int         `json:"nodePort"`
	NodeGrpcPort   int         `json:"nodeGrpcPort"`
	GrpcReportDir  string      `json:"grpcReportDir"`
	OemEbo         interface{} `json:"oemEbo"`
	OemAppID       interface{} `json:"oemAppId"`
	DutNodeMapList interface{} `json:"dutNodeMapList"`
	DutPCMapList   interface{} `json:"dutPCMapList"`
	MtbfDutList    interface{} `json:"mtbfDutList"`
	FileInfoMap    interface{} `json:"fileInfoMap"`
	BtnDisplayMap  interface{} `json:"btnDisplayMap"`
	FileInfoList   interface{} `json:"fileInfoList"`
	ForceGet       bool        `json:"forceGet"`
	UseProxy       bool        `json:"useProxy"`
}

// QueryDeviceInfoByDutID queries the device info by DUT ID.
func QueryDeviceInfoByDutID(dutID string, requestURL string, caseName string) (*DUTInfo, error) {
	return queryDeviceInfo(dutID, requestURL, caseName)
}

// QueryDeviceInfoByDeviceID queries the device info by Device ID.
func QueryDeviceInfoByDeviceID(deviceID string, requestURL string, caseName string) (*DUTInfo, error) {
	return queryDeviceInfo(deviceID, requestURL, caseName)
}

func queryDeviceInfo(id string, requestURL string, caseName string) (*DUTInfo, error) {

	//bodyBuf := "{\"id\":\"" + id + "\"}"
	bodyBuf := fmt.Sprintf(`{"id": %q, "caseName": %q}`, id, caseName)

	req, err := http.NewRequest(
		"POST",
		requestURL,
		strings.NewReader(bodyBuf))
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{Timeout: time.Second * 20}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.New("ID:" + id + " " + err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("[%s] [Code %d] [Raw Body: %s]", err, resp.StatusCode, string(bodyBytes))
		return nil, errors.New(errStr)
	}
	dutInfo := &DUTInfo{}
	if err = json.Unmarshal(bodyBytes, dutInfo); err != nil {
		errStr := fmt.Sprintf("[%s] [Code %d] [Raw Body: %s]", err, resp.StatusCode, string(bodyBytes))
		return nil, errors.New(errStr)
	}

	return dutInfo, nil
}
