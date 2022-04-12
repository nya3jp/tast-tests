// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/mod/semver"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BaseECUpdate,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that detachable base notification appears upon firmware update",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Data:         []string{"moonball_old_20220406.bin"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(
			hwdep.Model("kakadu"),
			hwdep.ChromeEC(),
		),
		Fixture: fixture.DevModeGBB,
	})
}

func BaseECUpdate(ctx context.Context, s *testing.State) {
	// To-do:
	// Currently at the experimental stage, this test is set up to run on Kukuki(Kakadu).
	// Our goal would be to expand to a wider range of DUTs, as defined in the following map:
	// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hammerd/hammertests/#prepare-host-and-dut
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	s.Log("Logging in to Chrome")
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	dut := s.DUT()
	ecTool := firmware.NewECTool(dut, firmware.ECToolNameMain)

	srcImgPath := s.DataPath("moonball_old_20220406.bin")
	dstImgPath := filepath.Join("/tmp", strings.Split(srcImgPath, "data/")[1])

	utilServiceClient := fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	s.Log("Flashing an old image to detachable-base ec")
	if err := flashAnOldImgToDetachableBaseEC(ctx, dut, utilServiceClient, srcImgPath, dstImgPath); err != nil {
		s.Fatal("Failed to flash base ec to an old version: ", err)
	}

	// Given that DUT's base ec is running an old firmware,
	// detaching then re-attaching base would trigger an update
	// notification window to pop up in a logged in session.
	if err := triggerAndFindNotification(ctx, ecTool, utilServiceClient); err != nil {
		s.Fatal("Failed to trigger and find notification: ", err)
	}

	s.Log("Saving the base ec firmware version after flashing an old image")
	oldRWVersion, err := getBaseECVersion(ctx, dut)
	if err != nil {
		s.Fatal("Failed to check base ec's version: ", err)
	}

	s.Log("Power-cycling DUT with a warm reset")
	h.CloseRPCConnection(ctx)
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatal("Failed to reboot DUT by servo: ", err)
	}
	s.Log("Waiting for DUT to power ON")
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()

	if err := s.DUT().WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	s.Log("Saving the base ec firmware version after reboot ")
	newRWVersion, err := getBaseECVersion(ctx, dut)
	if err != nil {
		s.Fatal("Failed to check base ec's version: ", err)
	}

	s.Log("Verifying that base ec updated after a reboot")
	if !verifyBaseECVersion(oldRWVersion, newRWVersion) {
		s.Fatal("Failed to update the base ec back to default version")
	}
}

func flashAnOldImgToDetachableBaseEC(ctx context.Context, dut *dut.DUT, utilsSvcClient fwpb.UtilsServiceClient, srcImg, dstImg string) error {
	// We downloaded an old base ec image from the following source:
	// https://chrome-internal-review.googlesource.com/c/chromeos/overlays/overlay-kukui-private/+/2743655
	// This file was pre-uploaded to Google Cloud as external data.
	_, err := linuxssh.PutFiles(ctx, dut.Conn(), map[string]string{srcImg: dstImg}, linuxssh.DereferenceSymlinks)
	if err != nil {
		return errors.Wrap(err, "failed to copy an old ec file downloaded from the Cloud to DUT")
	}

	crosCfgRes, err := utilsSvcClient.GetDetachableBaseValue(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get detachable-base attribute values")
	}

	if err := dut.Conn().CommandContext(
		ctx,
		"/sbin/minijail0", "-e", "-N", "-p", "-l", "-u",
		"hammerd", "-g", "hammerd", "-c", "0002", "/usr/bin/hammerd",
		"--ec_image_path="+dstImg,
		crosCfgRes.Values[0], crosCfgRes.Values[1], crosCfgRes.Values[2],
		"--update_if=always",
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to run the hammerd command")
	}

	return nil
}

func verifyBaseECVersion(old, new string) bool {
	return semver.Compare(old, new) == -1
}

func getBaseECVersion(ctx context.Context, dut *dut.DUT) (string, error) {
	// From our tested runs, 'hammer_info.py' only supports release versions greater than
	// or equal to R99. Relevant CL can be found as follows:
	// https://chromium-review.googlesource.com/c/chromiumos/platform2/+/3380089
	testing.ContextLog(ctx, "Running 'hammer_info.py' to read the base EC RW version")
	hammerInfoPath := "/usr/local/bin/hammer_info.py"
	out, err := dut.Conn().CommandContext(ctx, "python3", hammerInfoPath).Output(ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run the hammer_info file")
	}

	rawString := string(out)
	reg := regexp.MustCompile(`rw_version="\D+_`)
	match := reg.FindStringSubmatch(rawString)
	msg := strings.Split(rawString, match[0])
	rwVersion := strings.Split(msg[len(msg)-1], "-")

	if len(rwVersion[0]) == 0 {
		return "", errors.New("failed to extract the rw version")
	}
	return rwVersion[0], nil
}

func triggerAndFindNotification(ctx context.Context, ecTool *firmware.ECTool, utilSvcClient fwpb.UtilsServiceClient) error {

	// Included in baseGpioNames are a list of possible gpios available for
	// controlling the base state. The first one found from the list would
	// be used in setting base state attached/detached.
	var baseStateGpio string
	baseGpioNames := []firmware.GpioName{firmware.ENBASE, firmware.ENPP3300POGO}
	foundNames, err := ecTool.FindBaseGpio(ctx, baseGpioNames)

	if err != nil {
		return errors.Wrapf(err, "while looking for %q", baseGpioNames)
	}
	for _, name := range baseGpioNames {
		if _, ok := foundNames[name]; ok {
			baseStateGpio = string(name)
			break
		}
	}

	// Detach then re-attach detachable's base to trigger update notification.
	for _, step := range []struct {
		basestate string
		value     string
	}{
		{
			basestate: baseStateGpio,
			value:     "0",
		},
		{
			basestate: baseStateGpio,
			value:     "1",
		},
	} {
		if err := ecTool.Command(ctx, "gpioset", step.basestate, step.value).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to switch the basestate")
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep 1 second for the command to fully propagate to the DUT")
		}
	}

	testing.ContextLog(ctx, "Finding notification window")
	const title = "Your detachable keyboard needs a critical update"
	req := fwpb.NodeElement{
		Name: title,
	}
	if _, err := utilSvcClient.FindSingleNode(ctx, &req); err != nil {
		return errors.Wrap(err, "failed to find notification of detachable keyboard update")
	}

	return nil
}
