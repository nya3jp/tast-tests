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
	localnearby "chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/services/cros/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			nearbyshare.RegisterNearbyShareServiceServer(srv, &NearbyService{s: s})
		},
	})
}

// NearbyService implements tast.cros.nearbyshare.NearbyShareService.
type NearbyService struct {
	s *testing.ServiceState

	cr              *chrome.Chrome
	tconn           *chrome.TestConn
	deviceName      string
	senderSurface   *localnearby.SendSurface
	receiverSurface *localnearby.ReceiveSurface
}

// NewChromeLogin logs into Chrome with Nearby Share flags enabled.
func (n *NearbyService) NewChromeLogin(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr != nil {
		return nil, errors.New("Chrome already available")
	}
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
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
	return &empty.Empty{}, nil
}

// CloseChrome closes all surfaces and Chrome.
func (n *NearbyService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	os.RemoveAll(nearbytestutils.SendDir)
	n.senderSurface.Close(ctx)
	n.receiverSurface.Close(ctx)
	err := n.cr.Close(ctx)
	n.cr = nil
	return &empty.Empty{}, err
}

// CrOSSetup performs Nearby Share setup on a ChromeOS device.
func (n *NearbyService) CrOSSetup(ctx context.Context, req *nearbyshare.CrOSSetupRequest) (*empty.Empty, error) {
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
	return &empty.Empty{}, localnearby.StartHighVisibilityMode(ctx, n.tconn, n.deviceName)
}

// PrepareFilesAndStartSend extracts test files and then starts to share the extracted files.
func (n *NearbyService) PrepareFilesAndStartSend(ctx context.Context, req *nearbyshare.CrOSSendFilesRequest) (*nearbyshare.CrOSSendFilesResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, req.FileName)
	if err != nil {
		testing.ContextLog(ctx, "Failed to extract test files")
		return nil, err
	}
	var res nearbyshare.CrOSSendFilesResponse
	res.FileNames = filenames
	// Get the full paths of the test files to pass to chrome://nearby.
	var testFiles []string
	for _, f := range filenames {
		testFiles = append(testFiles, filepath.Join(nearbytestutils.SendDir, f))
	}
	sender, err := localnearby.StartSendFiles(ctx, n.cr, testFiles)
	if err != nil {
		testing.ContextLog(ctx, "Failed to set up control over the send surface")
		return nil, err
	}
	n.senderSurface = sender
	return &res, nil
}

// SelectShareTarget selects the expected receiver in the sending window.
func (n *NearbyService) SelectShareTarget(ctx context.Context, req *nearbyshare.CrOSSelectShareTargetRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if n.senderSurface == nil {
		return nil, errors.New("SendSurface is not defined")
	}
	return &empty.Empty{}, n.senderSurface.SelectShareTarget(ctx, req.ReceiverName, localnearby.CrosDetectReceiverTimeout)
}

// StartReceiving enables high vis mode receiving via Javascript.
func (n *NearbyService) StartReceiving(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	receiver, err := localnearby.StartReceiving(ctx, n.cr)
	if err != nil {
		return nil, errors.New("failed to set up control over the receiving surface")
	}
	n.receiverSurface = receiver
	return &empty.Empty{}, nil
}

// WaitForSenderAndAcceptShare is called by a receiver to wait for a sender to appear in their list and accepts the share from them.
func (n *NearbyService) WaitForSenderAndAcceptShare(ctx context.Context, req *nearbyshare.CrOSReceiveFilesRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if n.receiverSurface == nil {
		return nil, errors.New("ReceiveSurface is not defined")
	}
	if _, err := n.receiverSurface.WaitForSender(ctx, req.SenderName, localnearby.CrosDetectSenderTimeout); err != nil {
		return nil, errors.New("CrOS receiver failed to find CrOS sender")
	}
	if err := n.receiverSurface.AcceptShare(ctx); err != nil {
		return nil, errors.New("CrOs receiver failed to accept share from CrOS sender")
	}
	return &empty.Empty{}, nil
}

// GetFilesHashes takes some filenames and returns a list of their hashes.
func (n *NearbyService) GetFilesHashes(ctx context.Context, req *nearbyshare.CrOSFileHashRequest) (*nearbyshare.CrOSFileHashResponse, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	var res nearbyshare.CrOSFileHashResponse
	hashes, err := nearbytestutils.CrOSFileHashFilenames(ctx, req.FileNames, req.FileDir)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get hash of %s ", req.FileNames)
	}
	res.Hashes = hashes
	return &res, nil
}
