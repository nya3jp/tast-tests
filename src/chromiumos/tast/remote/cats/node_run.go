// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node/sdk"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

const errWithUsrErrCode = "5300"
const usrErrWithReturn = "9999"

type createTaskClosure func(ctx context.Context, deviceID string, catsClient *NodeClient) (string, *NodeErr)

// RunCaseParams contains the parameters for c-ats case run.
type RunCaseParams struct {
	HostName       string
	RequestURL     string
	CaseName       string
	CaseDesc       string
	TaskName       string
	DeviceID       string
	GrpcReportPath string
	NodeIP         string
	NodePort       int
	NodeGRPCPort   int
	query          bool
}

func (p *RunCaseParams) validate() *mtbferrors.MTBFError {
	if len(p.CaseName) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoCaseName, nil)
	}
	if len(p.CaseName) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoDUTName, nil)
	}
	if len(p.RequestURL) != 0 {
		p.query = true
		return nil
	}
	if len(p.DeviceID) == 0 || len(p.NodeIP) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoQueryURL, nil)
	}
	p.query = false
	return nil
}

//DeepCopy copies a RunCaseParams
func (p *RunCaseParams) DeepCopy() *RunCaseParams {
	return &RunCaseParams{
		HostName:     p.HostName,
		RequestURL:   p.RequestURL,
		CaseName:     p.CaseName,
		TaskName:     p.TaskName,
		DeviceID:     p.DeviceID,
		NodeIP:       p.NodeIP,
		NodePort:     p.NodePort,
		NodeGRPCPort: p.NodeGRPCPort,
		query:        p.query,
	}
}

// GetNodeClientAddress returns the node client address.
func GetNodeClientAddress(deviceHostName, requestURL string, caseName string) (string, *mtbferrors.MTBFError) {
	deviceInfo, err := getDeviceInfo(deviceHostName, requestURL, caseName)
	if err != nil {
		return "", mtbferrors.New(mtbferrors.CatsQueryFailure, err)
	}
	if len(deviceInfo.NodeIP) == 0 {
		return "", mtbferrors.New(mtbferrors.CatsNoNodeIP, nil)
	}
	if deviceInfo.NodeGrpcPort == 0 {
		return "", mtbferrors.New(mtbferrors.CatsNoNodeGrpcPort, nil)
	}

	addr := fmt.Sprintf("%s:%d", deviceInfo.NodeIP, deviceInfo.NodeGrpcPort)

	return addr, nil
}

func getDeviceInfo(deviceHostName string, requestURL string, caseName string) (*DUTInfo, *mtbferrors.MTBFError) {
	deviceInfo, err := QueryDeviceInfoByDutID(deviceHostName, requestURL, caseName)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CatsQueryFailure, err)
	}
	if len(deviceInfo.NodeIP) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodeIP, nil)
	}
	if len(deviceInfo.DeviceID) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoDUTID, nil)
	}

	return deviceInfo, nil
}

func convNode2MtbfErr(nodeErr *NodeErr,
	errCode mtbferrors.MTBFErrCode,
	taskID string,
	RptURL string,
	args ...interface{}) *mtbferrors.MTBFError {

	if len(nodeErr.ErrorCode) == 0 {
		return mtbferrors.New(errCode, nil, nodeErr.ErrorMessage, args)
	}
	return mtbferrors.NewCatsNodeErr(
		mtbferrors.CatsErrCode{
			MTBFErrCode:      &mtbferrors.CatsErr6011,
			TaskID:           taskID,
			TaskRptURL:       RptURL,
			CatsNodeOrigCode: nodeErr.ErrorCode},
		nil, nodeErr.ErrorMessage, args)
}

func runCase(ctx context.Context, params *RunCaseParams, createTask createTaskClosure) (interface{}, *mtbferrors.MTBFError) {

	deviceInfo := &DUTInfo{
		NodeIP:   params.NodeIP,
		NodePort: params.NodePort,
		DeviceID: params.DeviceID,
	}
	var mtbfErr *mtbferrors.MTBFError
	if params.query {
		deviceInfo, mtbfErr = getDeviceInfo(params.HostName, params.RequestURL, params.CaseName)
		if mtbfErr != nil {
			return nil, mtbfErr
		}
	}

	catsClient, mtbfErr := getNodeClient(deviceInfo.NodeIP, deviceInfo.NodePort)
	if mtbfErr != nil {
		return nil, mtbfErr
	}

	taskID, nodeErr := createTask(ctx, deviceInfo.DeviceID, catsClient)
	if nodeErr != nil {
		rptURL := catsClient.GetTaskReportURL(ctx, taskID, deviceInfo.DeviceID)
		return nil, convNode2MtbfErr(nodeErr, mtbferrors.CatsErr6007, taskID, rptURL)
	}

	return getTaskResult(ctx, catsClient, taskID, deviceInfo.DeviceID)
}

// RunSubCase runs CATS sub case.
func RunSubCase(ctx context.Context,
	runParams *RunCaseParams, caseFile string, caseParams interface{}) (*json.RawMessage, *mtbferrors.MTBFError) {

	if err := runParams.validate(); err != nil {
		return nil, err
	}
	caseName := runParams.CaseName
	createTask := func(ctx context.Context, deviceID string, catsClient *NodeClient) (string, *NodeErr) {

		taskID := time.Now().Format("20060102150405000") + "_" + caseName

		req := newSubCaseReq(taskID, deviceID, caseFile, caseName, caseParams)

		return taskID, catsClient.CreateSubCase(ctx, req)
	}

	result, mtbfErr := runCase(ctx, runParams, createTask)
	if result == nil {
		return nil, mtbfErr
	}
	return result.(*json.RawMessage), nil
}

// RunCase runs CATS case.
func RunCase(ctx context.Context, runParams *RunCaseParams) (*string, *mtbferrors.MTBFError) {
	if err := runParams.validate(); err != nil {
		return nil, err
	}
	caseName := runParams.CaseName
	createTask := func(ctx context.Context, deviceID string, catsClient *NodeClient) (string, *NodeErr) {

		taskID := time.Now().Format("20060102150405000") + "_" + caseName
		taskName := caseName
		taskPath := "Test_" + caseName + ".xml"

		return taskID, catsClient.CreateTask(ctx, taskID, taskName, taskPath, deviceID)
	}

	result, mtbfErr := runCase(ctx, runParams, createTask)
	if result == nil {
		return nil, mtbfErr
	}
	return result.(*string), nil
}

func getNodeClient(nodeIP string, nodePort int) (*NodeClient, *mtbferrors.MTBFError) {
	port := 6601
	if nodePort < 0 {
		return nil, mtbferrors.New(mtbferrors.CatsErr6012, nil, nodePort)
	} else if nodePort > 0 {
		port = nodePort
	}
	catsClient, err := NewNodeClient(nodeIP, port)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CatsErr6006, err)
	}
	return catsClient, nil
}

func getTaskResult(ctx context.Context, catsClient *NodeClient, taskID string, deviceID string) (interface{}, *mtbferrors.MTBFError) {

	timeout := time.Duration(time.Second * 600)
	startTime := time.Now()
	taskErrCnt := 0

	rptURL := catsClient.GetTaskReportURL(ctx, taskID, deviceID)

	for {

		if time.Now().Sub(startTime) > timeout {
			return nil, mtbferrors.NewCatsNodeErr(
				mtbferrors.CatsErrCode{
					MTBFErrCode: &mtbferrors.CatsErr6008,
					TaskID:      taskID,
					TaskRptURL:  rptURL,
				},
				nil, startTime, time.Now())
		}

		testing.Sleep(ctx, time.Duration(time.Second*2))

		taskInfo, nodeErr := catsClient.GetStatus(ctx, taskID, deviceID)
		if nodeErr != nil {
			taskErrCnt = taskErrCnt + 1
			// sometimes, CATS will try to re-connnect the ADB connection of the DUT in 15s
			if taskErrCnt > 15 {
				return nil, convNode2MtbfErr(nodeErr, mtbferrors.CatsErr6009, taskID, rptURL, startTime, time.Now())
			}
			continue
		}

		// reset the counter
		taskErrCnt = 0

		if taskInfo.TaskStatus == "FINISHED" {
			caseResult := taskInfo.CaseResults[0]
			// Task is finished
			if taskInfo.TaskStatistics.TotalCaseNum != taskInfo.TaskStatistics.PassedCaseNum {
				// All Fail case (failedCaseNum, errorCaseNum, tbcCaseNum, notRunCaseNum)
				if strings.Contains(caseResult.ErrorCode, errWithUsrErrCode) &&
					strings.Contains(caseResult.UserErrorCode, usrErrWithReturn) {
					// ErrorCode=5300 and UserErrorCode=9999, not fail, pass with return value
					return &caseResult.UserErrorMsg, nil
				} else if strings.Contains(caseResult.ErrorCode, errWithUsrErrCode) &&
					len(caseResult.UserErrorCode) > 0 {
					// ErrorCode=5300 and UserErrorCode not empty, fail with UserErrorCode
					// And replace the ErrorCode with UserErrorCode
					return nil, mtbferrors.NewCatsNodeErr(
						mtbferrors.CatsErrCode{
							MTBFErrCode:      &mtbferrors.CatsErr6011,
							TaskID:           taskID,
							TaskRptURL:       rptURL,
							CatsNodeOrigCode: caseResult.UserErrorCode,
						},
						nil, caseResult.ErrorMsg)
				} else {
					// Other fail case
					return nil, mtbferrors.NewCatsNodeErr(
						mtbferrors.CatsErrCode{
							MTBFErrCode:      &mtbferrors.CatsErr6011,
							TaskID:           taskID,
							TaskRptURL:       rptURL,
							CatsNodeOrigCode: caseResult.ErrorCode,
						},
						nil, caseResult.ErrorMsg)
				}
			}
			if len(caseResult.SubCaseReturn) > 0 &&
				string(caseResult.SubCaseReturn) != "null" {
				return &caseResult.SubCaseReturn, nil
			}
			return nil, nil
		}
	}
}

// DetachCaseRun runs a CATS case with Detach mode.
func DetachCaseRun(ctx context.Context, params *RunCaseParams, main, cleanUp sdk.Handler) (interface{}, error) {

	androidTest, err := sdk.New(fmt.Sprintf("%s:%d", params.NodeIP, params.NodeGRPCPort))
	if err != nil {
		// refer to mtbf/meta/common/common.go
		return nil, mtbferrors.New(mtbferrors.OSCreateNodeClient, err)
	}

	if main == nil {
		main = func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
			return nil, nil
		}
	}

	caseRpt, result, err := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        params.CaseName,
		Description: params.CaseDesc,
		ReportPath:  params.GrpcReportPath,
		DutID:       params.DeviceID,
	}, main, cleanUp)

	var error2return error

	if err != nil {
		// refer to mtbf/meta/common/common.go
		error2return = mtbferrors.New(mtbferrors.OSNodeSendRequest, err)
	}
	// using the orig error handing here
	if caseRpt != nil {
		if caseRpt.UserErrorCode != 0 {
			error2return = mtbferrors.NewCatsNodeErr(
				mtbferrors.CatsErrCode{
					MTBFErrCode:      &mtbferrors.CatsErr6011,
					TaskID:           params.TaskName,
					TaskRptURL:       "",
					CatsNodeOrigCode: fmt.Sprintf("%04d", caseRpt.UserErrorCode),
				},
				nil, caseRpt.ErrorMsg)
		} else if caseRpt.ErrorCode != 0 {
			error2return = mtbferrors.NewCatsNodeErr(
				mtbferrors.CatsErrCode{
					MTBFErrCode:      &mtbferrors.CatsErr6011,
					TaskID:           params.TaskName,
					TaskRptURL:       "",
					CatsNodeOrigCode: fmt.Sprintf("%04d", caseRpt.ErrorCode),
				},
				nil, caseRpt.ErrorMsg)
		}
	}

	return result, error2return
}
