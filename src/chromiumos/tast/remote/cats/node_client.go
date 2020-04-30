// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cats

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// NodeClient is a wrapper for CATS Node Client.
type NodeClient struct {
	Host       string
	Port       int
	ID         string
	httpClient http.Client
}

// CreateTaskReq is the request body for Creating task API.
type CreateTaskReq struct {
	WSClientID   string `json:"wsClientId"`
	TaskID       string `json:"taskId"`
	TaskPlan     string `json:"taskPlan"`
	TaskFilePath string `json:"taskFilePath"`
	DeviceID     string `json:"deviceId"`
}

// NodeErr is the error for Creating task API.
type NodeErr struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func newNodeErr(errorMessage string) *NodeErr {
	return &NodeErr{
		ErrorCode:    "",
		ErrorMessage: errorMessage,
	}
}

func genUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	//fmt.Println(uuid)
	return string(uuid), nil
}

// NewNodeClient returns a CATS None Client.
func NewNodeClient(host string, port int) (*NodeClient, error) {
	uuid, err := genUUID()
	if err != nil {
		return nil, err
	}
	return &NodeClient{host, port, uuid, http.Client{Timeout: time.Second * 20}}, nil
}

// CreateTask creates a CATS task.
func (catsClient *NodeClient) CreateTask(ctx context.Context, taskID string, taskPlan string, taskFilePath string, deviceID string) *NodeErr {

	reqBody := CreateTaskReq{
		WSClientID:   catsClient.ID,
		TaskID:       taskID,
		TaskPlan:     taskPlan,
		TaskFilePath: taskFilePath,
		DeviceID:     deviceID,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return newNodeErr(err.Error())
	}

	url := fmt.Sprintf("http://%s:%d/nms/node/task/localCreate", catsClient.Host, catsClient.Port)

	return catsClient.createTask(ctx, url, body)
}

// RptTime is report time.
type RptTime struct {
	time.Time
}

// UnmarshalJSON is a unmarshl method for RptTime.
func (ct *RptTime) UnmarshalJSON(b []byte) (err error) {
	timeStr := strings.Trim(string(b), "\"")
	if len(timeStr) == 0 {
		return
	}
	ct.Time, err = time.Parse("2006-01-02 15:04:05", timeStr)
	return
}

// RptTimeDuration is the report time duration.
type RptTimeDuration struct {
	time.Duration
}

// UnmarshalJSON is a unmarshl method for RptTimeDuration.
func (ctd *RptTimeDuration) UnmarshalJSON(b []byte) (err error) {

	timeStr := strings.Trim(string(b), "\"")
	if len(timeStr) == 0 {
		return
	}
	duration := strings.Replace(timeStr, ":", "h", 1)
	duration = strings.Replace(duration, ":", "m", 1)
	duration += "s"
	ctd.Duration, err = time.ParseDuration(duration)
	return
}

// TaskStatistics has the task statistics.
type TaskStatistics struct {
	TotalCaseNum   int         `json:"totalCaseNum"`
	PassedCaseNum  int         `json:"passedCaseNum"`
	FailedCaseNum  int         `json:"failedCaseNum"`
	ErrorCaseNum   int         `json:"errorCaseNum"`
	TBCCaseNum     int         `json:"tbcCaseNum"`
	NotRunCaseNum  int         `json:"notRunCaseNum"`
	PassRate       interface{} `json:"passRate"`
	PassPercentage string      `json:"passPercentage"`
}

// CaseResult is the case result.
type CaseResult struct {
	Index          int             `json:"index"`
	CaseID         string          `json:"caseId"`
	CaseResult     string          `json:"caseResult"`
	AssignedResult string          `json:"assignedResult"`
	CaseName       string          `json:"caseName"`
	Features       string          `json:"features"`
	StartTime      RptTime         `json:"startTime"`
	EndTime        RptTime         `json:"endTime"`
	ExecDuration   interface{}     `json:"execDuration"`
	ErrorMsg       string          `json:"errorMsg"`
	ErrorCode      string          `json:"errorCode"`
	UserErrorCode  string          `json:"userErrorCode"`
	UserErrorMsg   string          `json:"userErrorMsg"`
	Remark         string          `json:"remark"`
	ErrorType      interface{}     `json:"errorType"`
	SubCaseReturn  json.RawMessage `json:"subCaseReturn"`
}

// Task holds task element in Report.
type Task struct {
	Finished           bool            `json:"finished"`
	TaskID             string          `json:"taskId"`
	TaskPlan           string          `json:"taskPlan"`
	TaskStatus         string          `json:"taskStatus"`
	CurrentCaseName    string          `json:"currentCaseName"`
	DeviceID           string          `json:"deviceId"`
	DeviceTaskPath     string          `json:"deviceTaskPath"`
	StartTime          RptTime         `json:"startTime"`
	EndTime            RptTime         `json:"endTime"`
	ExecDuration       RptTimeDuration `json:"execDuration"`
	ExecEnvInfo        interface{}     `json:"execEnvInfo"`
	TaskStatistics     TaskStatistics  `json:"taskStatistics"`
	CaseResults        []CaseResult    `json:"caseResults"`
	ResultType         string          `json:"resultType"`
	TaskExtFuncResults interface{}     `json:"taskExtFuncResults"`
}

// TasksRpt has the Task Report structure.
type TasksRpt struct {
	Total       int    `json:"total"`
	PageSize    int    `json:"pageSize"`
	CurrentPage int    `json:"currentPage"`
	Tasks       []Task `json:"tasks"`
}

// GetStatus gets task status.
func (catsClient *NodeClient) GetStatus(ctx context.Context, taskID string, deviceID string) (*Task, *NodeErr) {

	requestURL := fmt.Sprintf("http://%s:%d/nms/attachment/%s/%s/task_rpt.json",
		catsClient.Host, catsClient.Port, taskID, deviceID)
	req, err := http.NewRequest(
		"GET",
		requestURL,
		nil)
	if err != nil {
		return nil, newNodeErr(err.Error())
	}
	req.WithContext(ctx)
	//req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("serverToken", catsClient.ID)
	req.Header.Set("taskID", taskID)
	req.Header.Set("OpenType", "OPEN")

	resp, err := catsClient.httpClient.Do(req)
	if err != nil {
		return nil, newNodeErr(err.Error())
	}
	defer resp.Body.Close()

	rspBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		if resp.StatusCode == 500 {
			createErr := &NodeErr{}
			if err := json.Unmarshal(rspBody, createErr); err != nil {
				errMsg := fmt.Sprintf("[%d] %s", resp.StatusCode, string(rspBody))
				return nil, newNodeErr(errMsg)
			}
			return nil, createErr
		}
		errMsg := fmt.Sprintf("[%d] %s", resp.StatusCode, string(rspBody))
		return nil, newNodeErr(errMsg)
	}
	taskInfo := &Task{}
	if err := json.Unmarshal(rspBody, taskInfo); err != nil {
		errMsg := fmt.Sprintf("[%d] %s", resp.StatusCode, string(rspBody))
		return nil, newNodeErr(errMsg)
	}
	return taskInfo, nil
}

// SubCaseReq is the request for running a CATS sub case.
type SubCaseReq struct {
	CaseType        string      `json:"caseType"`
	TaskID          string      `json:"taskId"`
	DeviceID        string      `json:"deviceId"`
	SubCaseFileName string      `json:"subCaseFileName"`
	SubCaseFunc     string      `json:"subCaseFunc"`
	SubCaseParam    interface{} `json:"subCaseParam"`
}

func newSubCaseReq(taskID string, deviceID string,
	subCaseFileName string, subCaseFunc string,
	subCaseParam interface{}) *SubCaseReq {
	return &SubCaseReq{
		CaseType:        "python",
		TaskID:          taskID,
		DeviceID:        deviceID,
		SubCaseFileName: subCaseFileName,
		SubCaseFunc:     subCaseFunc,
		SubCaseParam:    subCaseParam,
	}
}

func (catsClient *NodeClient) createTask(ctx context.Context, url string, reqBody []byte) *NodeErr {
	bodyBuf := bytes.NewBuffer([]byte(reqBody))

	req, err := http.NewRequest(
		"POST",
		url,
		bodyBuf)
	if err != nil {
		return newNodeErr(err.Error())
	}
	req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("serverToken", catsClient.ID)

	resp, err := catsClient.httpClient.Do(req)
	if err != nil {
		return newNodeErr(err.Error())
	}
	defer resp.Body.Close()

	rspBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 204 {
		if resp.StatusCode == 500 {
			createErr := &NodeErr{}
			if err := json.Unmarshal(rspBody, createErr); err != nil {
				errMsg := fmt.Sprintf("[%d] %s", resp.StatusCode, string(rspBody))
				return newNodeErr(errMsg)
			}
			return createErr
		}
		errMsg := fmt.Sprintf("[%d] %s", resp.StatusCode, string(rspBody))
		return newNodeErr(errMsg)
	}
	return nil
}

// CreateSubCase creates a CATS sub case task.
func (catsClient *NodeClient) CreateSubCase(ctx context.Context, reqBody *SubCaseReq) *NodeErr {

	body, err := json.Marshal(reqBody)
	if err != nil {
		return newNodeErr(err.Error())
	}
	url := fmt.Sprintf("http://%s:%d/nms/node/task/runSubcase", catsClient.Host, catsClient.Port)

	return catsClient.createTask(ctx, url, body)
}

// GetTaskReportURL gets the task report URL.
func (catsClient *NodeClient) GetTaskReportURL(ctx context.Context, taskID string, deviceID string) string {
	url := fmt.Sprintf("http://%s:%d/nms/case/report/%s/%s/task_rpt_%s.html",
		catsClient.Host, catsClient.Port,
		taskID, deviceID, taskID)
	return url
}
