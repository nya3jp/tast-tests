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
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/rpc"
	nearbyservice "chromiumos/tast/services/cros/nearbyshare"
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
		ServiceDeps:  []string{"tast.cros.nearbyshare.NearbyShareService"},
		Vars:         []string{"secondaryTarget"},
		Params: []testing.Param{
			{
				Name: "small_jpg",
				Val:  nearbytestutils.TestData{Filename: "small_jpg.zip", Timeout: nearbyshare.SmallFileTimeout},
				// TODO(crbug/1111251): Replace with external data when remote tests support it.
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.SmallFileTimeout,
			},
		},
	})
}

// CrosSenderCrosReceiver tests file sharing between Chrome OS devices.
func CrosSenderCrosReceiver(ctx context.Context, s *testing.State) {
	// TODO(b/175889133) Remove hardcoded hostnames when multi dut skylab support is available.
	const (
		HatchHostname   = "chromeos15-row6a-rack12-host2a"
		OctopusHostname = "chromeos15-row6a-rack12-host2b"
	)
	d1 := s.DUT()

	// Figure out which DUT is primary and which is secondary.
	// Switch on the DUTs in our lab setup first, then fall back to user supplied var.
	var secondaryDUT string
	if strings.Contains(s.DUT().HostName(), HatchHostname) {
		secondaryDUT = OctopusHostname
	} else if strings.Contains(s.DUT().HostName(), OctopusHostname) {
		secondaryDUT = HatchHostname
	} else {
		secondary, ok := s.Var("secondaryTarget")
		if !ok {
			s.Fatal("Test is running on an unknown hostname and no secondaryTarget arg was supplied")
		}
		secondaryDUT = secondary
	}

	s.Log("Ensuring we can connect to DUT2: ", secondaryDUT)
	d2, err := d1.NewSecondaryDevice(secondaryDUT)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}
	if err := d2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to secondary DUT: ", err)
	}

	s.Log("Preparing to send remote data files to DUT1")
	tempdir, err := d1.Conn().Command("mktemp", "-d", "/tmp/nearby_share_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	dataPath := strings.TrimSpace(string(tempdir))

	// Remove all files on both DUTs when test is done.
	defer d1.Conn().Command("rm", "-r", dataPath).Output(ctx)
	defer d2.Conn().Command("rm", "-r", filesapp.DownloadPath).Output(ctx)

	remoteFilePath := filepath.Join(dataPath, s.Param().(nearbytestutils.TestData).Filename)
	s.Log("Sending data files to DUT1: ", remoteFilePath)
	if _, err := linuxssh.PutFiles(ctx, d1.Conn(), map[string]string{
		s.DataPath(s.Param().(nearbytestutils.TestData).Filename): remoteFilePath,
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
	dut1DisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT1. Name: ", dut1DisplayName)
	ns1, err := enableNearbyShare(ctx, s, cl1, dut1DisplayName)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT1: ", err)
	}
	defer ns1.CloseChrome(ctx, &empty.Empty{})

	cl2, err := rpc.Dial(ctx, d2, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial rpc service on  DUT2: ", err)
	}
	defer cl2.Close(ctx)
	dut2DisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT2. Name: ", dut2DisplayName)
	ns2, err := enableNearbyShare(ctx, s, cl2, dut2DisplayName)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT2: ", err)
	}
	defer ns2.CloseChrome(ctx, &empty.Empty{})

	s.Log("Starting receiving on DUT2")
	if _, err := ns2.StartReceiving(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start receiving on DUT2: ", err)
	}

	s.Log("Starting sending on DUT1")
	fileReq := &nearbyservice.CrOSSendFileRequest{FileName: remoteFilePath}
	fileNames, err := ns1.PrepareFilesAndStartSend(ctx, fileReq)
	if err != nil {
		s.Fatal("Failed to start send on DUT1: ", err)
	}

	s.Log("Selecting DUT2's share target on DUT1")
	targetReq := &nearbyservice.CrOSSelectShareTargetRequest{ReceiverName: dut2DisplayName}
	if _, err := ns1.SelectShareTarget(ctx, targetReq); err != nil {
		s.Fatal("Failed to select share target on DUT1: ", err)
	}

	s.Log("Accepting the share request on DUT2")
	receiveReq := &nearbyservice.CrOSReceiveFilesRequest{SenderName: dut1DisplayName}
	if _, err := ns2.WaitForSenderAndAcceptShare(ctx, receiveReq); err != nil {
		s.Fatal("Failed to accept share on DUT2: ", err)
	}

	// Repeat the file hash check for a few seconds, as we have no indicator on the CrOS side for when the received file has been completely written.
	// TODO(crbug/1173190): Remove polling when we can confirm the transfer status with public test functions.
	s.Log("Comparing file hashes for all transferred files on both DUTs")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dut1FileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: nearbytestutils.SendDir}
		dut1FileRes, err := ns1.FilesHashes(ctx, dut1FileReq)
		if err != nil {
			return errors.Wrap(err, "failed to get file hashes on DUT1")
		}
		dut2FileReq := &nearbyservice.CrOSFileHashRequest{FileNames: fileNames.FileNames, FileDir: filesapp.DownloadPath}
		dut2FileRes, err := ns2.FilesHashes(ctx, dut2FileReq)
		if err != nil {
			return errors.Wrap(err, "failed to get file hashes on DUT2")
		}
		if len(dut1FileRes.Hashes) != len(dut2FileRes.Hashes) {
			return errors.Wrap(err, "length of hashes don't match")
		}
		for i := range dut1FileRes.Hashes {
			if dut1FileRes.Hashes[i] != dut2FileRes.Hashes[i] {
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
	if _, err := ns.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Setup Nearby Share on the DUT.
	req := &nearbyservice.CrOSSetupRequest{DataUsage: int32(nearbysetup.DataUsageOnline), Visibility: int32(nearbysetup.VisibilityAllContacts), DeviceName: deviceName}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}
	return ns, nil
}
