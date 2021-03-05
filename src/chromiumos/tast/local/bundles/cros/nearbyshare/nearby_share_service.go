// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			nearbyservice.RegisterNearbyShareServiceServer(srv, &NearbyService{s: s})
		},
	})
}

// NearbyService implements tast.cros.nearbyservice.NearbyShareService.
type NearbyService struct {
	s *testing.ServiceState

	cr              *chrome.Chrome
	tconn           *chrome.TestConn
	deviceName      string
	senderSurface   *nearbyshare.SendSurface
	receiverSurface *nearbyshare.ReceiveSurface
	chromeReader    *syslog.LineReader
	messageReader   *syslog.LineReader
}

// NewChromeLogin logs into Chrome with Nearby Share flags enabled.
func (n *NearbyService) NewChromeLogin(ctx context.Context, req *nearbyservice.CrOSLoginRequest) (*empty.Empty, error) {
	if n.cr != nil {
		return nil, errors.New("Chrome already available")
	}
	nearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	if req.Username != "" {
		nearbyOpts = append(nearbyOpts, chrome.Auth(req.Username, req.Password, ""), chrome.GAIALogin())
	}
	cr, err := chrome.New(ctx, nearbyOpts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}
	n.cr = cr
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get a connection to the Test Extension")
		return nil, err
	}
	n.tconn = tconn

	// Start collecting chrome and messages logs.
	chromeReader, err := nearbytestutils.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome logging")
	}
	messageReader, err := nearbytestutils.StartLogging(ctx, syslog.MessageFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed io start Message logging")
	}
	testing.ContextLog(ctx, "Started logging chrome and message logs")
	n.chromeReader = chromeReader
	n.messageReader = messageReader
	return &empty.Empty{}, nil
}

// CloseChrome closes all surfaces and Chrome.
// This will likely be called in a defer in remote tests instead of called explicitly. So log everything that fails to aid debugging later.
func (n *NearbyService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		testing.ContextLog(ctx, "Chrome not available")
		return nil, errors.New("Chrome not available")
	}
	os.RemoveAll(nearbytestutils.SendDir)
	if n.senderSurface != nil {
		if err := n.senderSurface.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Closing SendSurface failed: ", err)
		}
	}
	if n.receiverSurface != nil {
		if err := n.receiverSurface.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Closing ReceiveSurface failed: ", err)
		}
	}
	if err := os.RemoveAll(nearbyshare.NearbyLogDir); err != nil {
		testing.ContextLog(ctx, "Faied to delete nearby log dir: ", err)
	}
	if err := os.Mkdir(nearbyshare.NearbyLogDir, 0755); err != nil {
		testing.ContextLog(ctx, "Faied to create tmp dir log: ", err)
	}
	if err := nearbytestutils.SaveLogs(ctx, n.chromeReader, filepath.Join(nearbyshare.NearbyLogDir, nearbyshare.ChromeLog)); err != nil {
		testing.ContextLog(ctx, "Faied to save chrome log: ", err)
	}
	if err := nearbytestutils.SaveLogs(ctx, n.messageReader, filepath.Join(nearbyshare.NearbyLogDir, nearbyshare.MessageLog)); err != nil {
		testing.ContextLog(ctx, "Faied to save message log: ", err)
	}
	err := n.cr.Close(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Faied to close Chrome in Nearby Share service: ", err)
	} else {
		testing.ContextLog(ctx, "Nearby Share service closed successfully for: ", n.deviceName)
	}
	n.cr = nil
	return &empty.Empty{}, err
}

// CrOSSetup performs Nearby Share setup on a ChromeOS device.
func (n *NearbyService) CrOSSetup(ctx context.Context, req *nearbyservice.CrOSSetupRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	n.deviceName = req.DeviceName
	return &empty.Empty{}, nearbysetup.CrOSSetup(ctx, n.tconn, n.cr, nearbysetup.DataUsage(req.DataUsage), nearbysetup.Visibility(req.Visibility), req.DeviceName)
}

// StartHighVisibilityMode starts high vis mode using the UI library.
func (n *NearbyService) StartHighVisibilityMode(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	return &empty.Empty{}, nearbyshare.StartHighVisibilityMode(ctx, n.tconn, n.deviceName)
}

// PrepareFiles extracts test files.
func (n *NearbyService) PrepareFiles(ctx context.Context, req *nearbyservice.CrOSPrepareFileRequest) (*nearbyservice.CrOSPrepareFileResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, req.FileName)
	if err != nil {
		testing.ContextLog(ctx, "Failed to extract test files")
		return nil, err
	}
	res := nearbyservice.CrOSPrepareFileResponse{FileNames: filenames}
	return &res, nil
}

// StartSend starts to share files.
func (n *NearbyService) StartSend(ctx context.Context, req *nearbyservice.CrOSSendFilesRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if n.senderSurface != nil {
		n.senderSurface.Close(ctx)
	}

	// Get the full paths of the test files to pass to chrome://nearby.
	var testFiles []string
	for _, f := range req.FileNames {
		testFiles = append(testFiles, filepath.Join(nearbytestutils.SendDir, f))
	}
	sender, err := nearbyshare.StartSendFiles(ctx, n.cr, testFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up control over the send surface")
	}
	n.senderSurface = sender
	return &empty.Empty{}, nil
}

// SelectShareTarget selects the expected receiver in the sending window.
func (n *NearbyService) SelectShareTarget(ctx context.Context, req *nearbyservice.CrOSSelectShareTargetRequest) (*nearbyservice.CrOSShareTokenResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if n.senderSurface == nil {
		return nil, errors.New("SendSurface is not defined")
	}
	if err := n.senderSurface.SelectShareTarget(ctx, req.ReceiverName, nearbyshare.DetectShareTargetTimeout); err != nil {
		return nil, errors.Wrap(err, "failed to select share target")
	}
	var res nearbyservice.CrOSShareTokenResponse
	if req.CollectShareToken {
		token, err := n.senderSurface.ConfirmationToken(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get confirmation token")
		}
		res.ShareToken = token
	}
	return &res, nil
}

// StartReceiving enables high vis mode receiving via Javascript.
func (n *NearbyService) StartReceiving(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	receiver, err := nearbyshare.StartReceiving(ctx, n.tconn, n.cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up control over the receiving surface")
	}
	n.receiverSurface = receiver
	return &empty.Empty{}, nil
}

// WaitForSenderAndAcceptShare is called by a receiver to wait for a sender to appear in their list and accepts the share from them.
func (n *NearbyService) WaitForSenderAndAcceptShare(ctx context.Context, req *nearbyservice.CrOSReceiveFilesRequest) (*nearbyservice.CrOSShareTokenResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if n.receiverSurface == nil {
		return nil, errors.New("ReceiveSurface is not defined")
	}
	var res nearbyservice.CrOSShareTokenResponse
	token, err := n.receiverSurface.WaitForSender(ctx, req.SenderName, nearbyshare.DetectShareTargetTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "CrOS receiver failed to find CrOS sender")
	}
	res.ShareToken = token
	if err := n.receiverSurface.AcceptShare(ctx); err != nil {
		return nil, errors.Wrap(err, "CrOS receiver failed to accept share from CrOS sender")
	}
	return &res, nil
}

// FilesHashes takes some filenames and returns a list of their hashes.
func (n *NearbyService) FilesHashes(ctx context.Context, req *nearbyservice.CrOSFileHashRequest) (*nearbyservice.CrOSFileHashResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	var res nearbyservice.CrOSFileHashResponse
	hashes, err := nearbytestutils.HashFiles(ctx, req.FileNames, req.FileDir)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get hash of %s ", req.FileNames)
	}
	res.Hashes = hashes
	return &res, nil
}

// AcceptIncomingShareNotificationAndWaitForCompletion accepts the incoming transfer via notification. Used for in contact tests.
func (n *NearbyService) AcceptIncomingShareNotificationAndWaitForCompletion(ctx context.Context, req *nearbyservice.CrOSReceiveFilesRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if err := nearbyshare.AcceptIncomingShareNotification(ctx, n.tconn, req.SenderName, nearbyshare.DetectShareTargetTimeout); err != nil {
		return nil, errors.Wrap(err, "CrOS receiver failed to accept Nearby Share notification")
	}
	testing.ContextLog(ctx, "Accepted the share on the CrOS receiver")
	testing.ContextLog(ctx, "Waiting for receiving-complete notification on CrOS receiver")
	if err := nearbyshare.WaitForReceivingCompleteNotification(ctx, n.tconn, req.SenderName, nearbyshare.DetectShareTargetTimeout); err != nil {
		return nil, errors.Wrap(err, "failed waiting for notification to indicate sharing has completed on CrOS")
	}
	return &empty.Empty{}, nil
}
