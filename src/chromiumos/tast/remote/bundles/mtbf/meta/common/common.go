// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node/sdk"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats"
	"chromiumos/tast/testing"
)

// DutID is a key to get real target device ID.
const DutID cats.ContextKeyType = `DutID`

// CompanionID is the CATS test companion device ID.
const CompanionID = `${deviceId1}`

// CatsMTBFLogin calls a subtest to login on the DUT for CATS testing.
func CatsMTBFLogin(ctx context.Context, s *testing.State) error {
	flags := GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "cats.MTBFLogin"); err != nil {
		return err
	}

	return nil
}

// CatsNodeAddress gets the CATS node address from give service url.
func CatsNodeAddress(ctx context.Context, s *testing.State, caseName string) (string, error) {
	urlVar := "cats.requestURL"
	requestURL, urlOk := s.Var(urlVar)
	nodeIP, ipOk := s.CheckVar("cats.nodeIP")
	nodePort, portOk := s.CheckVar("cats.nodePort")

	// if URL is given, use it.
	if urlOk {
		addr, err := cats.GetNodeClientAddress(s.DUT().GetHostname(), requestURL, caseName)
		if err != nil {
			return "", err
		}

		return addr, nil
	}

	// if URL is not given, try IP and port.
	if !ipOk || !portOk {
		// complain URL is not given.
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, urlVar))
	}

	return fmt.Sprintf("%s:%s", nodeIP, nodePort), nil
}

// GetFlags checks userID and userPasswd and get flags
func GetFlags(s *testing.State) []string {
	// userID and userPasswd must be explicitly provided for running sub-tests
	if _, ok := s.CheckVar("userID"); !ok {
		s.Fatal("userID not provided")
	}
	if _, ok := s.CheckVar("userPasswd"); !ok {
		s.Fatal("userPasswd not provided")
	}
	flags := []string{
		"-build=false",
	}
	return flags
}

// GetDetachedFlags gets detached flags
func GetDetachedFlags(s *testing.State, duration int) []string {
	flags := []string{
		"-build=false",
		"-localbundledir=/usr/local/libexec/tast/bundles/local_pushed",
		"-localdatadir=/usr/local/share/tast/data_pushed",
		"-localrunner=/usr/local/libexec/tast/bin_pushed/local_test_runner",
		fmt.Sprintf("-detachduration=%d", duration),
	}
	return flags
}

// DriveDUT runs local case
func DriveDUT(ctx context.Context, s *testing.State, caseName string) error {
	flags := GetFlags(s)
	if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, caseName); mtbferr != nil {
		return mtbferr
	}

	return nil
}

// Cleanup is called for defer to cleanup test.
func Cleanup(ctx context.Context, s *testing.State, err error, flags []string, testName string) {
	if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, testName); err == nil && mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// PollDetachedCaseDone checks if detached a sub case is finished.
func PollDetachedCaseDone(ctx context.Context, s *testing.State, statusSvr string, dutID string, caseName string) error {
	if err := testing.Poll(ctx, func(context.Context) error {
		detachedCaseDone, err := checkDetachedCaseDone(ctx, s, statusSvr, dutID, caseName)
		s.Log("detachedCaseDone: ", detachedCaseDone)

		if err != nil {
			return err
		} else if !detachedCaseDone {
			return errors.New("detached case is not finished yet")
		}

		s.Log("detached case is finished. Will enable ethernet")

		return nil
	}, &testing.PollOptions{Interval: 10 * time.Second, Timeout: 12 * time.Minute}); err != nil {
		return err
	}

	return nil
}

func checkDetachedCaseDone(ctx context.Context, s *testing.State, statusSvr string, dutID string, testCase string) (bool, error) {
	statusURL := fmt.Sprintf("%v?action=status&dut=%v&testCase=%v", statusSvr, dutID, testCase)
	s.Log("statusURL: ", statusURL)
	response, statusCode, err := httputil.HTTPGetStr(statusURL, 30*time.Second)

	if err != nil {
		s.Log("Unable to get response from status servlet: ", err)
		return false, err
	}

	s.Logf("statusCode=%v, Response from status servlet: %q", statusCode, response)

	if strings.Contains(response, "Finished") {
		return true, nil
	}

	return false, nil
}

// EnableEthernet enables ethernet through Allion API.
func EnableEthernet(ctx context.Context, s *testing.State, allionServerURL string, deviceID string) {
	s.Log("Enabling ethernet through Allion API")
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	mtbferr := allionAPI.EnableEthernetWithRetry(deviceID, 3)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// NodeDetachModeRunCase calls C-ATS node to run android test.
func NodeDetachModeRunCase(ctx context.Context, s *testing.State, caseDesc sdk.CaseDescription, testRun, cleanUp sdk.Handler) interface{} {
	if err := CatsMTBFLogin(ctx, s); err != nil {
		s.Fatal("Failed to do MTBFLogin: ", err)
	}

	params, err := GetCatsRunParams(ctx, s, caseDesc.Name)
	if err != nil {
		s.Fatal("CATS run parameter failed: ", err)
	}

	caseDesc.DutID = params.DeviceID
	caseDesc.ReportPath = params.GrpcReportPath

	addr := fmt.Sprintf("%s:%d", params.NodeIP, params.NodeGRPCPort)

	androidTest, err := sdk.New(addr)
	if err != nil {
		s.Fatal("Failed to new android test: ", err)
	}

	ctx = context.WithValue(ctx, DutID, params.DeviceID)

	result, res, err := androidTest.RunDelegate(ctx, caseDesc, testRun, cleanUp)

	if err != nil {
		if _, ok := err.(sdk.Error); !ok {
			s.Fatal(mtbferrors.New(mtbferrors.OSNodeSendRequest, err))
		}
	}

	if result.Status == sdk.Failed {
		if result.UserErrorCode == 0 {
			s.Fatalf("[ERR-%d] "+result.ErrorMsg, result.ErrorCode)
		} else {
			s.Fatalf("[ERR-%d] "+result.UserErrorMsg, result.UserErrorCode)
		}
	}

	return res
}

// Sleep sleep a period of time and do error handling
func Sleep(ctx context.Context, s *testing.State, duration time.Duration) {
	if err := testing.Sleep(ctx, duration); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.WIFISleep, err))
	}
}

// GetCatsRunParams Get some parms for running a case
func GetCatsRunParams(ctx context.Context, s *testing.State, caseName string) (*cats.RunCaseParams, error) {
	hostName := s.DUT().GetHostname()
	if len(hostName) == 0 {
		return nil, mtbferrors.New(mtbferrors.OSVarRead, nil, "testing.State.DUT.GetHostname")
	}
	s.Logf("DUT Host Name [%s]", hostName)

	//deviceID, err := s.DUT().GetARCDeviceID(ctx)
	//if err != nil {
	//	return nil, mtbferrors.New(mtbferrors.OSVarRead, err, "testing.State.DUT.GetARCDeviceID")
	//}
	//s.Logf("DUT ARC Device ID [%s]", deviceID)

	varName := "cats.requestURL"
	requestURL, urlOk := s.Var(varName)
	if !urlOk {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, varName))
	}
	s.Logf("The Request URL [%s]", requestURL)

	deviceInfo, err := cats.QueryDeviceInfoByDutID(hostName, requestURL, caseName)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CatsQueryFailure, err)
	}
	if len(deviceInfo.DeviceID) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsDeviceID, nil)
	}
	s.Logf("The DUT ARC Device ID [%s]", deviceInfo.DeviceID)
	if len(deviceInfo.NodeIP) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsDeviceID, nil)
	}
	s.Logf("The CATS IP [%s]", deviceInfo.NodeIP)
	if deviceInfo.NodePort == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodePort, nil)
	}
	s.Logf("The CATS Port [%d]", deviceInfo.NodePort)
	if deviceInfo.NodeGrpcPort == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodeGrpcPort, nil)
	}
	s.Logf("The CATS GRPC Port [%d]", deviceInfo.NodeGrpcPort)
	if deviceInfo.GrpcReportDir == "" {
		return nil, mtbferrors.New(mtbferrors.CatsReportPath, nil)
	}
	s.Logf("The CATS Report Path [%s]", deviceInfo.GrpcReportDir)

	params := &cats.RunCaseParams{
		HostName:       hostName,
		RequestURL:     requestURL,
		DeviceID:       deviceInfo.DeviceID,
		NodeIP:         deviceInfo.NodeIP,
		NodePort:       deviceInfo.NodePort,
		NodeGRPCPort:   deviceInfo.NodeGrpcPort,
		CaseName:       caseName,
		GrpcReportPath: deviceInfo.GrpcReportDir,
	}
	return params, nil
}
