// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare contains fixtures meta tests use.
package nearbyshare

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/cros/nearbyshare"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// NewNearbyShareFixture creates a fixture for Nearby Share tests in different configurations.
func NewNearbyShareFixture(dataUsage nearbysetup.DataUsage, visibility nearbysetup.Visibility) testing.FixtureImpl {
	return &nearbyShareFixture{
		dataUsage:  dataUsage,
		visibility: visibility,
		// TODO(crbug/1127165): Remove after data is supported in fixture.
		testFiles: []string{"small_jpg.zip", "small_png.zip", "big_txt.zip"},
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOfflineAllContacts",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOfflineSomeContacts",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'Some Contacts' with the sender selected as a contact on the receiver side",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilitySelectedContacts),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOnlineAllContacts",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilityAllContacts),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOnlineSomeContacts",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'Some Contacts' with the sender selected as a contact on the receiver side",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilitySelectedContacts),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOnlineNoOne",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilityNoOne),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareRemoteDataUsageOfflineNoOne",
		Desc: "Fixture for Nearby Share's CB -> CB tests. Each DUT is signed in with a real GAIA account that are in each other's contacts. Configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'No One'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityNoOne),
		Vars: []string{
			"secondaryTarget",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"nearbyshare.cros2_username",
			"nearbyshare.cros2_password",
			nearbycommon.KeepStateVar,
		},
		ServiceDeps:     []string{"tast.cros.nearbyservice.NearbyShareService"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareFixture struct {
	dataUsage  nearbysetup.DataUsage
	visibility nearbysetup.Visibility
	testFiles  []string

	// Sender and receiver devices.
	d1 *dut.DUT
	d2 *dut.DUT

	// These two vars actually have type *nearbyservice.nearbyShareServiceClient
	sender   nearbyservice.NearbyShareServiceClient
	receiver nearbyservice.NearbyShareServiceClient

	// Path on the sender where the test files are stored.
	remoteFilePath string

	// Attributes for both chromebooks.
	attributes []byte

	// Commands for starting/stopping bluetooth HCI logging.
	senderBtsnoopCmd   *ssh.Cmd
	receiverBtsnoopCmd *ssh.Cmd
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	RemoteFilePath      string
	SenderDisplayName   string
	ReceiverDisplayName string
	Sender              nearbyservice.NearbyShareServiceClient
	Receiver            nearbyservice.NearbyShareServiceClient
}

// Setup logs in, enables Nearby Share and moves test files to Sender DUT.
func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	d1 := s.DUT()
	f.d1 = d1
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
	f.d2 = d2
	s.Log("Preparing to move remote data files to DUT1 (Sender)")
	tempdir, err := d1.Conn().Command("mktemp", "-d", "/tmp/nearby_share_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	remoteDir := strings.TrimSpace(string(tempdir))
	f.remoteFilePath = remoteDir

	// TODO(crbug/1127165): Remove after data is supported in fixture.
	// Workaround to use data files downloaded in other tests.
	const (
		prebuiltLocalDataPath = "/usr/local/tast/data/chromiumos/tast/remote/bundles/cros/nearbyshare/data"
		builtLocalDataPath    = "../platform/tast-tests/src/chromiumos/tast/remote/bundles/cros/nearbyshare/data"
	)
	pathToUse := builtLocalDataPath
	// Use the built local data path if it exists, and fall back to the prebuilt data path otherwise.
	testFileCheck := filepath.Join(builtLocalDataPath, f.testFiles[0])
	if _, err := os.Stat(testFileCheck); os.IsNotExist(err) {
		pathToUse = prebuiltLocalDataPath
	} else if err != nil {
		s.Fatal("Failed to check if built local data path exists: ", err)
	}
	s.Log("Moving data files to DUT1 (Sender)")
	for _, data := range f.testFiles {
		remoteFilePath := filepath.Join(remoteDir, data)
		if _, err := linuxssh.PutFiles(ctx, d1.Conn(), map[string]string{filepath.Join(pathToUse, data): remoteFilePath}, linuxssh.DereferenceSymlinks); err != nil {
			s.Fatalf("Failed to send data to remote data path %v: %v", remoteDir, err)
		}
	}

	var keepState bool
	if val, ok := s.Var(nearbycommon.KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", nearbycommon.KeepStateVar, err)
		}
		keepState = b
	}

	// Login and setup Nearby Share on DUT 1 (Sender).
	cl1, err := rpc.Dial(s.FixtContext(), d1, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	const crosBaseName = "cros_test"
	senderDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT1 (Sender). Name: ", senderDisplayName)
	senderUsername := s.RequiredVar("nearbyshare.cros_username")
	senderPassword := s.RequiredVar("nearbyshare.cros_password")
	sender, err := f.enableNearbyShare(ctx, s, cl1, senderDisplayName, senderUsername, senderPassword, "", keepState)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT1 (Sender): ", err)
	}
	f.sender = sender

	// Login and setup Nearby Share on DUT 2 (Receiver).
	cl2, err := rpc.Dial(s.FixtContext(), d2, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial rpc service on DUT2: ", err)
	}
	receiverDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	s.Log("Enabling Nearby Share on DUT2 (Receiver). Name: ", receiverDisplayName)
	receiverUsername := s.RequiredVar("nearbyshare.cros2_username")
	receiverPassword := s.RequiredVar("nearbyshare.cros2_password")
	receiver, err := f.enableNearbyShare(ctx, s, cl2, receiverDisplayName, receiverUsername, receiverPassword, senderUsername, keepState)
	if err != nil {
		s.Fatal("Failed to enable Nearby Share on DUT2 (Receiver): ", err)
	}
	f.receiver = receiver
	// Get DUT attributes for both DUTs.
	senderAttrsRes, err := f.sender.CrOSAttributes(ctx, &empty.Empty{})
	if err != nil {
		s.Error("Failed to save device attributes on the sender: ", err)
	}
	receiverAttrsRes, err := f.receiver.CrOSAttributes(ctx, &empty.Empty{})
	if err != nil {
		s.Error("Failed to save device attributes on the receiver: ", err)
	}
	var senderAttributes *nearbysetup.CrosAttributes
	var receiverAttributes *nearbysetup.CrosAttributes
	if err := json.Unmarshal([]byte(senderAttrsRes.Attributes), &senderAttributes); err != nil {
		s.Error("Failed to unmarshal sender's attributes: ", err)
	}
	if err := json.Unmarshal([]byte(receiverAttrsRes.Attributes), &receiverAttributes); err != nil {
		s.Error("Failed to unmarshal receiver's: ", err)
	}
	attributes := struct {
		Sender   *nearbysetup.CrosAttributes
		Receiver *nearbysetup.CrosAttributes
	}{Sender: senderAttributes, Receiver: receiverAttributes}
	crosLog, err := json.MarshalIndent(attributes, "", "\t")
	if err != nil {
		s.Fatal("Failed to format device metadata for logging: ", err)
	}
	f.attributes = crosLog

	return &FixtData{
		RemoteFilePath:      f.remoteFilePath,
		Sender:              f.sender,
		Receiver:            f.receiver,
		SenderDisplayName:   senderDisplayName,
		ReceiverDisplayName: receiverDisplayName,
	}
}

// enableNearbyShare is a helper function to enable Nearby Share on each DUT.
// senderUsername is only used when the device visibility is "Some contacts".
// keepState is used to optionally preserve user accounts on the DUT.
// Sender devices should pass an empty string since the visibility setting is only relevant to receivers.
func (f *nearbyShareFixture) enableNearbyShare(ctx context.Context, s *testing.FixtState, cl *rpc.Client, deviceName, username, password, senderUsername string, keepState bool) (nearbyservice.NearbyShareServiceClient, error) {
	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	ns := nearbyservice.NewNearbyShareServiceClient(cl.Conn)
	loginReq := &nearbyservice.CrOSLoginRequest{Username: username, Password: password, KeepState: keepState}
	if _, err := ns.NewChromeLogin(ctx, loginReq); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Setup Nearby Share on the DUT.
	req := &nearbyservice.CrOSSetupRequest{DataUsage: int32(f.dataUsage), Visibility: int32(f.visibility), DeviceName: deviceName, SenderUsername: senderUsername}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}
	return ns, nil
}

// TearDown removes the test files from the sender and and closes the services on both DUTs.
func (f *nearbyShareFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Delete the test files from the sender.
	if err := f.d1.Conn().Command("rm", "-r", f.remoteFilePath).Run(ctx); err != nil {
		s.Error("Failed to remove test files from the sender: ", err)
	}
	// Shut down the nearby share service connections.
	if _, err := f.sender.CloseChrome(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to shutdown nearby share service connection for sender: ", err)
	}
	if _, err := f.receiver.CloseChrome(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to shutdown nearby share service connections for receiver: ", err)
	}
}

func (f *nearbyShareFixture) Reset(ctx context.Context) error { return nil }

// PreTest is run before each test in the fixture..
func (f *nearbyShareFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "device_attributes.json"), f.attributes, 0644); err != nil {
		s.Fatal("Failed to write CrOS attributes to output file: ", err)
	}

	// Start logging on each DUT.
	// TODO(crbug/1205689): move btsnoop log capture into the local NearbyService.
	if _, err := f.sender.StartLogging(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to save nearby share logs on the sender: ", err)
	}
	f.senderBtsnoopCmd = f.d1.Conn().Command("/usr/bin/btmon", "-w", filepath.Join("/tmp", nearbyshare.BtsnoopLog))
	if err := f.senderBtsnoopCmd.Start(s.TestContext()); err != nil {
		s.Error("Failed to start btsnoop log collection on the sender: ", err)
	}

	if _, err := f.receiver.StartLogging(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to save nearby share logs on the receiver: ", err)
	}
	f.receiverBtsnoopCmd = f.d2.Conn().Command("/usr/bin/btmon", "-w", filepath.Join("/tmp", nearbyshare.BtsnoopLog))
	if err := f.receiverBtsnoopCmd.Start(s.TestContext()); err != nil {
		s.Error("Failed to start btsnoop log collection on the receiver: ", err)
	}
}

// PostTest will pull the logs from the DUT and delete leftover logs and test files.
func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Save logs on each DUT.
	f.senderBtsnoopCmd.Abort()
	f.senderBtsnoopCmd = nil
	f.receiverBtsnoopCmd.Abort()
	f.receiverBtsnoopCmd = nil
	if _, err := f.sender.SaveLogs(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to save nearby share logs on the sender: ", err)
	}
	if _, err := f.receiver.SaveLogs(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to save nearby share logs on the receiver: ", err)
	}
	// Pull the log files back to the host.
	logsToSave := []string{nearbyshare.ChromeLog, nearbyshare.MessageLog, nearbyshare.BtsnoopLog}
	duts := []*dut.DUT{f.d1, f.d2}
	tags := []string{"sender", "receiver"}
	for i, dut := range duts {
		logFiles, err := dut.Conn().Command("ls", nearbyshare.NearbyLogDir).Output(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Failed to get list of log files in remote DUTs nearby temp dir: ", err)
		} else {
			testing.ContextLog(ctx, "Files in remote DUTs nearby temp dir: ", strings.Replace(string(logFiles), "\n", " ", -1))
		}
		for _, log := range logsToSave {
			logPathSrc := filepath.Join(nearbyshare.NearbyLogDir, log)
			logPathDst := filepath.Join(s.OutDir(), log+"_"+tags[i])
			if err := linuxssh.GetFile(ctx, dut.Conn(), logPathSrc, logPathDst, linuxssh.PreserveSymlinks); err != nil {
				testing.ContextLogf(ctx, "Failed to save %s to %s. Error: %s", logPathSrc, logPathDst, err)
			}
		}
		// Delete the log files so that we have a clean run for parameterized tests.
		if err := dut.Conn().Command("rm", "-r", nearbyshare.NearbyLogDir).Run(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to remove the log files at end of test: ", err)
		}
	}
	// Delete the sent files from the Downloads folder on the receiver.
	if _, err := f.receiver.ClearTransferredFiles(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to clear transferred files from the Downloads folder on the receiver: ", err)
	}
}
