// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	remotenearby "chromiumos/tast/remote/cros/nearbyshare"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosToCrosHighVis,
		Desc:         "Checks we can successfully send files from one Cros device to another",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share-remote"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyservice.NearbyShareService"},
		Vars:         []string{"secondaryTarget"},
		Params: []testing.Param{
			{
				Name:      "dataoffline_allcontacts_png5kb",
				Fixture:   "nearbyShareRemoteDataUsageOfflineAllContactsTestUser",
				Val:       nearbytestutils.TestData{Filename: "small_png.zip", TransferTimeout: nearbyshare.SmallFileTransferTimeout},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
			{
				Name:      "dataoffline_allcontacts_jpg11kb",
				Fixture:   "nearbyShareRemoteDataUsageOfflineAllContactsTestUser",
				Val:       nearbytestutils.TestData{Filename: "small_jpg.zip", TransferTimeout: nearbyshare.SmallFileTransferTimeout},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
			{
				Name:    "dataonline_noone_txt30mb",
				Fixture: "nearbyShareRemoteDataUsageOnlineNoOneGAIA",
				Val: nearbytestutils.TestData{
					Filename: "big_txt.zip", TransferTimeout: nearbyshare.LargeFileOnlineTransferTimeout},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

// CrosToCrosHighVis tests file sharing between Chrome OS devices.
func CrosToCrosHighVis(ctx context.Context, s *testing.State) {
	remoteFilePath := s.FixtValue().(*remotenearby.FixtData).RemoteFilePath
	sender := s.FixtValue().(*remotenearby.FixtData).Sender
	receiver := s.FixtValue().(*remotenearby.FixtData).Receiver
	senderDisplayName := s.FixtValue().(*remotenearby.FixtData).SenderDisplayName
	receiverDisplayName := s.FixtValue().(*remotenearby.FixtData).ReceiverDisplayName

	s.Log("Starting receiving on DUT2 (Receiver)")
	if _, err := receiver.StartReceiving(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start receiving on DUT2 (Receiver): ", err)
	}

	s.Log("Starting sending on DUT1 (Sender)")
	testData := s.Param().(nearbytestutils.TestData)
	remoteFile := filepath.Join(remoteFilePath, testData.Filename)
	fileReq := &nearbyservice.CrOSPrepareFileRequest{FileName: remoteFile}
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
	transferTimeoutSeconds := int32(testData.TransferTimeout.Seconds())
	receiveReq := &nearbyservice.CrOSReceiveFilesRequest{SenderName: senderDisplayName, TransferTimeoutSeconds: transferTimeoutSeconds}
	receiverShareToken, err := receiver.WaitForSenderAndAcceptShare(ctx, receiveReq)
	if err != nil {
		s.Fatal("Failed to accept share on DUT2 (Receiver): ", err)
	}
	if senderShareToken.ShareToken != receiverShareToken.ShareToken {
		s.Fatalf("Share tokens for sender and receiver do not match. Sender: %s, Receiver: %s", senderShareToken, receiverShareToken)
	}

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
