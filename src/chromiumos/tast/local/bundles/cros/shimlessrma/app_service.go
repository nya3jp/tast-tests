// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shimlessrma

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/shimlessrmaapp"
	"chromiumos/tast/local/shill"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
)

const (
	testFile              = "/var/lib/rmad/.test"
	offlineLogFile        = "/var/lib/rmad/offline.log"
	offlineExecuteSuccess = "Success"
	wifiName              = "GoogleGuest-IPv4"
	googleURL             = "google.com"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterAppServiceServer(srv, &AppService{s: s})
		},
	})
}

// AppService contains context about shimless rma.
type AppService struct {
	s   *testing.ServiceState
	cr  *chrome.Chrome
	app *shimlessrmaapp.RMAApp
}

// NewShimlessRMA creates ShimlessRMA.
func (shimlessRMA *AppService) NewShimlessRMA(ctx context.Context,
	req *pb.NewShimlessRMARequest) (*empty.Empty, error) {

	// If Reconnect is true, it means UI restarting during Shimless RMA testing.
	// Then, we don't need to stop rmad or create empty state file.
	if !req.Reconnect {
		// Make sure rmad is not currently running.
		// Ignore the error since ramd may not run at all.
		testexec.CommandContext(ctx, "stop", "rmad").Run()

		// Create a valid empty rmad state file.
		if err := shimlessrmaapp.CreateEmptyStateFile(); err != nil {
			return nil, errors.Wrap(err, "failed to create rmad state file")
		}
	}

	// Restart will also remove test file.
	// Therefore, we need to create it every time when we new ShimlessRMA
	if _, err := os.Create(testFile); err != nil {
		return nil, errors.Wrap(err, "failed to create .test file")
	}

	cr, err := chrome.New(ctx, chrome.EnableFeatures("ShimlessRMAFlow"),
		chrome.EnableFeatures("ShimlessRMAOsUpdate"),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req.ManifestKey),
		chrome.ExtraArgs("--launch-rma"))
	if err != nil {
		return nil, errors.Wrap(err, "Fail to new Chrome")
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	app, err := shimlessrmaapp.App(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Shimless RMA app")
	}

	shimlessRMA.cr = cr
	shimlessRMA.app = app

	return &empty.Empty{}, nil
}

// CloseShimlessRMA closes and releases the resources obtained by New.
func (shimlessRMA *AppService) CloseShimlessRMA(ctx context.Context,
	req *empty.Empty) (*empty.Empty, error) {
	// Ignore failure handle in this method,
	// since we want to execute all of these anyway.
	shimlessRMA.app.WaitForStateFileDeleted()(ctx)

	testexec.CommandContext(ctx, "stop", "rmad").Run()

	shimlessrmaapp.RemoveStateFile()

	os.Remove(testFile)

	shimlessRMA.cr.Close(ctx)

	return &empty.Empty{}, nil
}

// PrepareOfflineTest prepare DUT for offline test (temporary local test).
func (shimlessRMA *AppService) PrepareOfflineTest(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// Ignore error since file may not exist.
	os.Remove(offlineLogFile)

	// If we cannot create it, then we can catch it when we try to fetch this file.
	file, err := os.Create(offlineLogFile)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to create %s", offlineLogFile)
	}
	defer file.Close()

	return &empty.Empty{}, nil
}

// TestWelcomeAndNetworkConnection tests welcome page and network page in local test mode.
// We turn off ethernet in this method, so we cannot pass the error back to the gRPC caller.
// Instead, we write error (or Success) to a temp log and Host will verify the content of log later.
func (shimlessRMA *AppService) TestWelcomeAndNetworkConnection(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	file, _ := os.OpenFile(offlineLogFile, os.O_RDWR, 0644)
	defer file.Close()

	if err := action.Combine("test welcome page and network connection page in offline mode",
		shimlessRMA.handleUntilWifiConnectionWithoutEthernet,
		shimlessRMA.cancelShimlessRMA,
	)(ctx); err != nil {
		file.WriteString(err.Error())
		return nil, err
	}

	file.WriteString(offlineExecuteSuccess)
	return &empty.Empty{}, nil
}

// VerifyTestWelcomeAndNetworkConnectionSuccess verify that TestWelcomeAndNetworkConnection runs successfully.
// It reads offlineLogFile and verify the content.
func (shimlessRMA *AppService) VerifyTestWelcomeAndNetworkConnectionSuccess(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	content, err := ioutil.ReadFile(offlineLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "TestWelcomeAndNetworkConnection failed because we cannot read log file")
	}

	if string(content) != offlineExecuteSuccess {
		errorMessage := fmt.Sprintf("TestWelcomeAndNetworkConnection failed. Reason is %s", content)
		return nil, errors.New(errorMessage)
	}

	if err := os.Remove(offlineLogFile); err != nil {
		return nil, errors.Wrap(err, "fail to delete offline log file")
	}

	return &empty.Empty{}, nil
}

// VerifyNoWifiConnected verify that no wifi is connected.
func (shimlessRMA *AppService) VerifyNoWifiConnected(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	wifi, err := shill.NewWifiManager(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manager proxy")
	}
	// Disable ethernet and wifi to ensure the tethered connection is being used.
	connected, err := wifi.Connected(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Fail to check whether wifi is connected")
	}

	if connected {
		return nil, errors.New("Fail to forget wifi")
	}
	testing.ContextLog(ctx, "No wifi is connected")

	return &empty.Empty{}, nil
}

// WaitForPageToLoad waits the page with title to be loaded.
func (shimlessRMA *AppService) WaitForPageToLoad(ctx context.Context,
	req *pb.WaitForPageToLoadRequest) (*empty.Empty, error) {
	waitTimeout := time.Duration(req.DurationInSecond) * time.Second
	if err := shimlessRMA.app.WaitForPageToLoad(req.Title, waitTimeout)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to load page: %s", req.Title)
	}

	return &empty.Empty{}, nil
}

// LeftClickButton left clicks the button with label.
func (shimlessRMA *AppService) LeftClickButton(ctx context.Context,
	req *pb.LeftClickButtonRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickButton(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// WaitUntilButtonEnabled waits for button with label enabled.
func (shimlessRMA *AppService) WaitUntilButtonEnabled(ctx context.Context,
	req *pb.WaitUntilButtonEnabledRequest) (*empty.Empty, error) {
	waitTimeout := time.Duration(req.DurationInSecond) * time.Second
	if err := shimlessRMA.app.WaitUntilButtonEnabled(req.Label, waitTimeout)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// LeftClickRadioButton clicks radio button.
func (shimlessRMA *AppService) LeftClickRadioButton(ctx context.Context,
	req *pb.LeftClickRadioButtonRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickRadioButton(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click radio button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// LeftClickLink clicks link.
func (shimlessRMA *AppService) LeftClickLink(ctx context.Context,
	req *pb.LeftClickLinkRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickLink(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click link: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// RetrieveTextByPrefix returns the text with prefix.
func (shimlessRMA *AppService) RetrieveTextByPrefix(ctx context.Context,
	req *pb.RetrieveTextByPrefixRequest) (*pb.RetrieveTextByPrefixResponse, error) {
	node, err := shimlessRMA.app.RetrieveTextByPrefix(ctx, req.Prefix)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find info with prefix: %s", req.Prefix)
	}

	return &pb.RetrieveTextByPrefixResponse{Value: node.Name}, nil
}

// EnterIntoTextInput enters content into text input.
func (shimlessRMA *AppService) EnterIntoTextInput(ctx context.Context,
	req *pb.EnterIntoTextInputRequest) (*empty.Empty, error) {

	if err := shimlessRMA.app.EnterIntoTextInput(ctx, req.TextInputName, req.Content)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to enter content %s into text input", req.Content)
	}
	return &empty.Empty{}, nil
}

// BypassFirmwareInstallation add "firmware_updated":true to state file to bypass firmware installation.
func (shimlessRMA *AppService) BypassFirmwareInstallation(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	const stateFilePath string = "/mnt/stateful_partition/unencrypted/rma-data/state"
	jsonFile, err := os.Open(stateFilePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(byteValue), &result); err != nil {
		return nil, err
	}

	result["firmware_updated"] = true
	updatedByteValue, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(stateFilePath, updatedByteValue, 0666); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (shimlessRMA *AppService) handleWelcomePage(ctx context.Context) error {
	return action.Combine("Navigate to handle Welcome page",
		shimlessRMA.app.WaitForPageToLoad("Chromebook repair", time.Minute),
		shimlessRMA.app.WaitUntilButtonEnabled("Get started", time.Minute),
		shimlessRMA.app.LeftClickButton("Get started"),
	)(ctx)
}

func (shimlessRMA *AppService) connectWifiAndVerifyInternetConnection(ctx context.Context) error {
	return action.Combine("Fail to click wifi",
		shimlessRMA.app.WaitForPageToLoad("Get connected", time.Minute),
		shimlessRMA.app.LeftClickGenericContainer(wifiName),
		uiauto.Sleep(5*time.Second),
		shimlessRMA.app.LeftClickButton("Next"),
	)(ctx)
}

func (shimlessRMA *AppService) handleUntilWifiConnectionWithoutEthernet(ctx context.Context) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating shill manager proxy")
	}
	// Disable ethernet and wifi to ensure the tethered connection is being used.
	ethEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		return errors.Wrap(err, "Unable to disable ethernet")
	}
	defer ethEnableFunc(cleanupCtx)

	if networkAvailable(ctx) {
		return errors.New("Still access to Internet after ethernet is disabled")
	}

	return action.Combine("Handle until Wifi Connection without Ethernet",
		shimlessRMA.handleWelcomePage,
		shimlessRMA.connectWifiAndVerifyInternetConnection,
		shimlessRMA.app.WaitForPageToLoad("Select which components were replaced", time.Minute),
	)(ctx)
}

func (shimlessRMA *AppService) cancelShimlessRMA(ctx context.Context) error {
	return action.Combine("Cancel shimless RMA app",
		shimlessRMA.app.LeftClickButton("Exit"),
		shimlessRMA.app.LeftClickButton("Exit"),
	)(ctx)
}

func networkAvailable(ctx context.Context) bool {
	out, err := testexec.CommandContext(ctx, "ping", "-c", "3", googleURL).Output(testexec.DumpLogOnError)
	// Ping is expected to return an error if it fails to ping the server.
	// Just log the error and return false.
	if err != nil {
		testing.ContextLog(ctx, "Failed to run 'ping' command: ", err)
		return false
	}
	return strings.Contains(string(out), "3 received")
}
