// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cienet.com/cats/node/sdk"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

// DutID is a key to get real target device ID.
const DutID cats.ContextKeyType = `DutID`

// CompanionID is the CATS test companion device ID.
const CompanionID = `${deviceId1}`

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
func PollDetachedCaseDone(ctx context.Context, s *testing.State, statusSvr, dutID, caseName string) error {
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

func checkDetachedCaseDone(ctx context.Context, s *testing.State, statusSvr, dutID, testCase string) (bool, error) {
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
func EnableEthernet(ctx context.Context, s *testing.State, allionServerURL, deviceID string) {
	s.Log("Enabling ethernet through Allion API")
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	mtbferr := allionAPI.EnableEthernetWithRetry(deviceID, 3)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// NodeDetachModeRunCase calls C-ATS node to run android test.
func NodeDetachModeRunCase(ctx context.Context, s *testing.State, caseDesc sdk.CaseDescription, testRun, cleanUp sdk.Handler) interface{} {
	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer c.Close(ctx)

	commSvcClient := svc.NewCommServiceClient(c.Conn)

	localOutDir := s.RPCHint().LocalOutDir
	//if _, err := commSvcClient.LoginWithARC(ctx, &svc.LoginRequest{OutDir: localOutDir}); err != nil {
	//  s.Fatal(mtbferrors.New(mtbferrors.ChromeArcLogin, err))
	//}
	if _, err := commSvcClient.Login(ctx, &svc.LoginRequest{OutDir: localOutDir}); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeLogin, err))
	}
	// Close shall be called to cleanup the login session.
	defer func() {
		if _, err := commSvcClient.Close(ctx, &empty.Empty{}); err != nil {
			s.Log("Login Close error: ", err)
		}
	}()

	params, err := GetCatsRunParams(ctx, s, caseDesc.Name)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CatsParameter, err))
	}

	caseDesc.DutID = params.DeviceID
	caseDesc.ReportPath = params.GrpcReportPath
	caseDesc.LogFolderPath = params.GrpcDroneLogDir

	addr := fmt.Sprintf("%s:%d", params.NodeIP, params.NodeGRPCPort)

	testing.ContextLog(ctx, "Connecting to Node server")
	androidTest, err := sdk.New(addr)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CatsAndroidTest, err))
	}
	defer androidTest.Close()

	testing.ContextLog(ctx, "Device ID injection")
	ctx = context.WithValue(ctx, DutID, params.DeviceID)

	testing.ContextLog(ctx, "Start running case in delegate mode")
	result, res, err := androidTest.RunDelegate(ctx, caseDesc, testRun, cleanUp)

	if err != nil {
		serr, ok := err.(sdk.Error)
		if !ok {
			s.Fatal(mtbferrors.New(mtbferrors.OSNodeSendRequest, err))
		} else if result == nil {
			s.Fatal(mtbferrors.New(mtbferrors.OSSDKResultEmpty, serr))
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

	varName := "meta.requestURL"
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
	if deviceInfo.GrpcReportDir == "" {
		return nil, mtbferrors.New(mtbferrors.CatsReportPath, nil)
	}
	s.Logf("The CATS Report Path [%s]", deviceInfo.GrpcReportDir)

	params := &cats.RunCaseParams{
		HostName:        hostName,
		RequestURL:      requestURL,
		DeviceID:        deviceInfo.DeviceID,
		NodeIP:          deviceInfo.NodeIP,
		NodePort:        deviceInfo.NodePort,
		NodeGRPCPort:    deviceInfo.NodeGrpcPort,
		CaseName:        caseName,
		GrpcReportPath:  deviceInfo.GrpcReportDir,
		GrpcDroneLogDir: deviceInfo.GrpcDroneLogDir,
	}
	return params, nil
}

// Fatal log the error message and take screenshot
func Fatal(ctx context.Context, s *testing.State, mtbferr error) {
	go func(s *testing.State) {
		takeScreenshots(ctx, s)
	}(s)
	time.Sleep(time.Second * 5) // NOLINT
	s.Fatal(mtbferr)
}

func takeScreenshots(ctx context.Context, s *testing.State) {
	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Log(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer c.Close(ctx)

	commSvcClient := svc.NewCommServiceClient(c.Conn)
	_, err = commSvcClient.TakeScreenshot(ctx, &empty.Empty{})
	if err != nil {
		s.Log(mtbferrors.New(mtbferrors.GRPCTakeScreenshot, err))
	}

	dir, err := getDir(ctx, "screenshots")
	if err != nil {
		s.Log(mtbferrors.New(mtbferrors.GRPCScreenshot, err))
	}

	d := s.DUT()
	dutFilePath := "/home/chronos/user/Downloads/screenshot_chrome.png"
	fileName := "screenshot_chrome_" + time.Now().Format("20060102-150405") + ".png"

	if err := d.GetFile(ctx, dutFilePath, filepath.Join(dir, fileName)); err != nil {
		s.Log(mtbferrors.New(mtbferrors.GRPCScreenshot, err))
	}
}

func getDir(ctx context.Context, dirName string) (string, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	// If test setup failed, then the output dir may not exist.
	if !ok || outDir == "" {
		return "", errors.New("outDir is not set")
	}
	if _, err := os.Stat(outDir); err != nil {
		return "", errors.New("outDir does not exist")
	}

	dir := filepath.Join(outDir, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", errors.New("cannot make " + dirName + " directory")
	}
	return dir, nil
}
