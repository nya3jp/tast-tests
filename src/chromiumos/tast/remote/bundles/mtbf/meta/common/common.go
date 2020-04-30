// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats"
	"chromiumos/tast/testing"
)

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

// CatsNodeAddress get the CATS node address from give service url.
func CatsNodeAddress(ctx context.Context, s *testing.State) (string, error) {
	urlVar := "cats.requestURL"
	requestURL, urlOk := s.Var(urlVar)
	nodeIP, ipOk := s.CheckVar("cats.nodeIP")
	nodePort, portOk := s.CheckVar("cats.nodePort")

	// if URL is given, use it.
	if urlOk {
		addr, err := cats.GetNodeClientAddress(s.DUT().GetHostname(), requestURL)
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

// GetFlags check userID and userPasswd and get flags
func GetFlags(s *testing.State) []string {
	// userID and userPasswd must be explictly provided for running sub-tests
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

// GetDetachedFlags get detached flags
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

// DriveDUT run local case
func DriveDUT(ctx context.Context, s *testing.State, caseName string) error {
	flags := GetFlags(s)
	if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, caseName); mtbferr != nil {
		return mtbferr
	}

	return nil
}

// Cleanup for defer
func Cleanup(ctx context.Context, s *testing.State, err error, flags []string, testName string) {
	if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, testName); err == nil && mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// PollDetachedCaseDone check if detached a sub case is finished
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
	resp, err := http.Get(statusURL)

	if err != nil {
		s.Log("Failed to get detached case status: ", err)
		return false, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		s.Log("Unable to get response from status servlet: ", err)
		return false, err
	}

	response := strings.TrimSuffix(string(body), "\n")
	s.Log("Response from status servlet: '", response, "'")

	if strings.Contains(response, "Finished") {
		return true, nil
	}

	return false, nil
}

// EnableEthernet enable ethernet through Allion API
func EnableEthernet(ctx context.Context, s *testing.State, allionServerURL string, deviceID string) {
	s.Log("Enabling ethernet through Allion API")
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	mtbferr := allionAPI.EnableEthernet(deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// GetCatsRunParams Get some parms for runing a case
func GetCatsRunParams(ctx context.Context, s *testing.State) (*cats.RunCaseParams, error) {
	hostName := s.DUT().GetHostname()
	if len(hostName) == 0 {
		return nil, mtbferrors.New(mtbferrors.OSVarRead, nil, "testing.State.DUT.GetHostname")
	}
	s.Logf("DUT Host Name [%s]", hostName)

	deviceID, err := s.DUT().GetARCDeviceID(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.OSVarRead, err, "testing.State.DUT.GetARCDeviceID")
	}
	s.Logf("DUT ARC Device ID [%s]", deviceID)

	varName := "cats.requestURL"
	requestURL, urlOk := s.Var(varName)
	if !urlOk {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, varName))
	}
	s.Logf("The Request URL [%s]", requestURL)

	deviceInfo, err := cats.QueryDeviceInfoByDutID(hostName, requestURL)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CatsQueryFailure, err)
	}
	if len(deviceInfo.NodeIP) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodeIP, nil)
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

	params := &cats.RunCaseParams{
		HostName:     hostName,
		RequestURL:   requestURL,
		DeviceID:     deviceID,
		NodeIP:       deviceInfo.NodeIP,
		NodePort:     deviceInfo.NodePort,
		NodeGRPCPort: deviceInfo.NodeGrpcPort,
	}
	return params, nil
}
