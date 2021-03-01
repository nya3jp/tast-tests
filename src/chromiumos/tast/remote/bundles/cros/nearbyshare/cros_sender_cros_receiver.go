// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/remote/bundles/cros/nearbyshare/remotetestutils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosSenderCrosReceiver,
		Desc:         "Checks we can successfully send files from one Cros device to another",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyservice.NearbyShareService"},
		Vars:         []string{"secondaryTarget"},
		Params: []testing.Param{
			{
				Name:      "small_jpg",
				Val:       nearbytestutils.TestData{Filename: "small_jpg.zip", Timeout: nearbyshare.SmallFileTimeout},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.SmallFileTimeout,
			},
		},
	})
}

// CrosSenderCrosReceiver tests file sharing between Chrome OS devices.
func CrosSenderCrosReceiver(ctx context.Context, s *testing.State) {
	d1 := s.DUT()
	secondary, ok := s.Var("secondaryTarget")
	if !ok {
		secondary = ""
	}
	secondaryDUT, err := nearbytestutils.ChooseSecondaryDUT(d1.HostName(), secondary)
	if err != nil {
		s.Fatal("Failed to find hostname for DUT2: ", err)
	}
	s.Log("Ensuring we can connect to DUT2: ", secondaryDUT)
	d2, err := d1.NewSecondaryDevice(secondaryDUT)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}
	if err := d2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to secondary DUT: ", err)
	}

	s.Log("Preparing to move remote data files to DUT1 (Sender)")
	tempdir, err := d1.Conn().Command("mktemp", "-d", "/tmp/nearby_share_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	dataPath := strings.TrimSpace(string(tempdir))
	defer d1.Conn().Command("rm", "-r", dataPath).Run(ctx)

	testData := s.Param().(nearbytestutils.TestData).Filename
	remoteFilePath := filepath.Join(dataPath, testData)
	s.Log("Moving data files to DUT1 (Sender): ", remoteFilePath)
	if _, err := linuxssh.PutFiles(ctx, d1.Conn(), map[string]string{
		s.DataPath(testData): remoteFilePath,
	}, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", dataPath, err)
	}

	// Login and setup Nearby Share on both DUTs
	cl1, err := rpc.Dial(ctx, d1, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl1.Close(ctx)
	const crosBaseName = "cros_test"
	senderDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT1 (Sender). Name: ", senderDisplayName)
	sender, err := enableNearbyShare(ctx, s, cl1, senderDisplayName)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT1 (Sender): ", err)
	}
	defer remotetestutils.SaveLogs(ctx, sender, d1, "sender", s.OutDir())
	cl2, err := rpc.Dial(ctx, d2, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial rpc service on DUT2: ", err)
	}
	defer cl2.Close(ctx)
	receiverDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT2 (Receiver). Name: ", receiverDisplayName)
	receiver, err := enableNearbyShare(ctx, s, cl2, receiverDisplayName)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT2 (Receiver): ", err)
	}
	defer remotetestutils.SaveLogs(ctx, receiver, d1, "receiver", s.OutDir())

	s.Log("Starting receiving on DUT2 (Receiver)")
	if _, err := receiver.StartReceiving(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start receiving on DUT2 (Receiver): ", err)
	}

	s.Log("Starting sending on DUT1 (Sender)")
	fileReq := &nearbyservice.CrOSPrepareFileRequest{FileName: remoteFilePath}
	fileNames, err := sender.PrepareFiles(ctx, fileReq)
	if err != nil {
		s.Fatal("Failed to prepare files for sending on DUT1 (Sender): ", err)
	}
	sendReq := &nearbyservice.CrOSSendFilesRequest{FileNames: fileNames.FileNames}
	_, err = sender.StartSend(ctx, sendReq)
	if err != nil {
		s.Fatal("Failed to start send on DUT1 (Sender): ", err)
	}

	s.Log("Selecting Receiver's (DUT2) share target on Sender (DUT1)")
	targetReq := &nearbyservice.CrOSSelectShareTargetRequest{ReceiverName: receiverDisplayName, CollectShareToken: true}
	senderShareToken, err := sender.SelectShareTarget(ctx, targetReq)
	if err != nil {
		s.Fatal("Failed to select share target on DUT1 (Sender): ", err)
	}
	s.Log("Accepting the share request on DUT2 (Receiver)")
	receiveReq := &nearbyservice.CrOSReceiveFilesRequest{SenderName: senderDisplayName}
	receiverShareToken, err := receiver.WaitForSenderAndAcceptShare(ctx, receiveReq)
	if err != nil {
		s.Fatal("Failed to accept share on DUT2 (Receiver): ", err)
	}
	if senderShareToken.ShareToken != receiverShareToken.ShareToken {
		s.Fatalf("Share tokens for sender and receiver do not match. Sender: %s, Receiver: %s", senderShareToken, receiverShareToken)
	}
	// Remove the files on the receiver after test run is complete.
	defer func() {
		for _, f := range fileNames.FileNames {
			filesPath := filepath.Join(filesapp.DownloadPath, f)
			d2.Conn().Command("rm", "-r", filesPath).Run(ctx)
		}
	}()

	// Repeat the file hash check for a few seconds, as we have no indicator on the CrOS side for when the received file has been completely written.
	// TODO(crbug/1173190): Remove polling when we can confirm the transfer status with public test functions.
	s.Log("Comparing file hashes for all transferred files on both DUTs")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		senderFileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: nearbytestutils.SendDir}
		senderFileRes, err := sender.FilesHashes(ctx, senderFileReq)
		if err != nil {
			return errors.Wrap(err, "failed to get file hashes on DUT1 (Sender)")
		}
		receiverFileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: filesapp.DownloadPath}
		receiverFileRes, err := receiver.FilesHashes(ctx, receiverFileReq)
		if err != nil {
			return errors.Wrap(err, "failed to get file hashes on DUT2 (Receiver)")
		}
		if len(senderFileRes.Hashes) != len(receiverFileRes.Hashes) {
			return errors.Wrap(err, "length of hashes don't match")
		}
		for i := range senderFileRes.Hashes {
			if senderFileRes.Hashes[i] != receiverFileRes.Hashes[i] {
				return errors.Wrap(err, "hashes don't match")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both DUTs")
}

// enableNearbyShare is a helper function to enable Nearby Share on each DUT.
func enableNearbyShare(ctx context.Context, s *testing.State, cl *rpc.Client, deviceName string) (nearbyservice.NearbyShareServiceClient, error) {
	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	ns := nearbyservice.NewNearbyShareServiceClient(cl.Conn)
	loginReq := &nearbyservice.CrOSLoginRequest{}
	if _, err := ns.NewChromeLogin(ctx, loginReq); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Setup Nearby Share on the DUT.
	req := &nearbyservice.CrOSSetupRequest{DataUsage: int32(nearbysetup.DataUsageOnline), Visibility: int32(nearbysetup.VisibilityAllContacts), DeviceName: deviceName}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}
	return ns, nil
}
