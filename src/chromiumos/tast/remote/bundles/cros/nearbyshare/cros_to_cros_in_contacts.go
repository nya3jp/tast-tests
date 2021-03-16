// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	remotenearby "chromiumos/tast/remote/cros/nearbyshare"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosToCrosInContacts,
		Desc:         "Checks we can successfully send files from one Cros device to another when they are in each other's contacts list",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share-remote"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyservice.NearbyShareService"},
		Params: []testing.Param{
			{
				Name:      "dataoffline_allcontacts_png5kb",
				Fixture:   "nearbyShareRemoteDataUsageOfflineAllContactsGAIA",
				Val:       nearbytestutils.TestData{Filename: "small_png.zip", TransferTimeout: nearbyshare.SmallFileTransferTimeout},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
			{
				Name:      "dataoffline_allcontacts_jpg11kb",
				Fixture:   "nearbyShareRemoteDataUsageOfflineAllContactsGAIA",
				Val:       nearbytestutils.TestData{Filename: "small_jpg.zip", TransferTimeout: nearbyshare.SmallFileTransferTimeout},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
			{
				Name:    "dataonline_allcontacts_txt30mb",
				Fixture: "nearbyShareRemoteDataUsageOnlineAllContactsGAIA",
				Val: nearbytestutils.TestData{
					Filename: "big_txt.zip", TransferTimeout: nearbyshare.LargeFileOnlineTransferTimeout},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

// CrosToCrosInContacts tests file sharing between Chrome OS devices where the users are contacts.
func CrosToCrosInContacts(ctx context.Context, s *testing.State) {
	remoteFilePath := s.FixtValue().(*remotenearby.FixtData).RemoteFilePath
	sender := s.FixtValue().(*remotenearby.FixtData).Sender
	receiver := s.FixtValue().(*remotenearby.FixtData).Receiver
	senderDisplayName := s.FixtValue().(*remotenearby.FixtData).SenderDisplayName
	receiverDisplayName := s.FixtValue().(*remotenearby.FixtData).ReceiverDisplayName

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
	targetReq := &nearbyservice.CrOSSelectShareTargetRequest{ReceiverName: receiverDisplayName, CollectShareToken: false}
	_, err = sender.SelectShareTarget(ctx, targetReq)
	if err != nil {
		s.Fatal("Failed to select share target on DUT1 (Sender): ", err)
	}

	s.Log("Accepting the share request on DUT2 (Receiver) via a notification")
	transferTimeoutSeconds := int32(testData.TransferTimeout.Seconds())
	receiveReq := &nearbyservice.CrOSReceiveFilesRequest{SenderName: senderDisplayName, TransferTimeoutSeconds: transferTimeoutSeconds}
	_, err = receiver.AcceptIncomingShareNotificationAndWaitForCompletion(ctx, receiveReq)
	if err != nil {
		s.Fatal("Failed to accept share on DUT2 (Receiver): ", err)
	}

	s.Log("Comparing file hashes for all transferred files on both DUTs")
	senderFileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: nearbytestutils.SendDir}
	senderFileRes, err := sender.FilesHashes(ctx, senderFileReq)
	if err != nil {
		s.Fatal("Failed to get file hashes on DUT1 (Sender): ", err)
	}
	receiverFileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: filesapp.DownloadPath}
	receiverFileRes, err := receiver.FilesHashes(ctx, receiverFileReq)
	if err != nil {
		s.Fatal("Failed to get file hashes on DUT2 (Receiver): ", err)
	}
	if len(senderFileRes.Hashes) != len(receiverFileRes.Hashes) {
		s.Fatal("Length of file hashes don't match")
	}
	for i := range senderFileRes.Hashes {
		if senderFileRes.Hashes[i] != receiverFileRes.Hashes[i] {
			s.Fatalf("Hashes don't match. Wanted: %s, Got: %s", senderFileRes.Hashes[i], receiverFileRes.Hashes[i])
		}
	}
	s.Log("Share completed and file hashes match on both DUTs")
}
