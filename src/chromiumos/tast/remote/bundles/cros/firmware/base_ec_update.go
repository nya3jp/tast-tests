// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/mod/semver"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	fwpb "chromiumos/tast/services/cros/firmware"
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
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(
			hwdep.Model("soraka", "krane", "kakadu", "katsu", "homestar", "mrbland", "wormdingler", "quackingstick"),
			hwdep.ChromeEC(),
		),
		Fixture: fixture.DevModeGBB,
		Timeout: 15 * time.Minute,
	})
}

// baseECInfo contains information about base ec.
type baseECInfo struct {
	name           string
	version        string
	protectionFlag string
	roProtected    bool
}

// modifiedFileDir contains paths defined as follows,
// onLocal: path on the local machine.
// onHost: path on DUT, where the modified base ec bin file will be copied to.
type modifiedFileDir struct {
	onLocal string
	onHost  string
}

// hammerRequiredVariables contains the required values for hammer command.
type hammerRequiredVariables struct {
	pid     string
	vid     string
	usbPath string
}

func BaseECUpdate(ctx context.Context, s *testing.State) {
	// To-do:
	// Our goal would be to expand to a wider range of DUTs, as defined in the following map:
	// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hammerd/hammertests/#prepare-host-and-dut
	// We noticed that Soraka and Nocturne don't have product-id, vendor-id, and usb-path,
	// which are the required params used in flashing the base ec bin file. Also, on Coachz
	// and Krane leased from the lab, even though the base ec version changed, the notification window did not
	// appear. We'll need to do some more research on these models. But at the moment, when tested in our office,
	// Krane [Google_Krane.12573.271.0] passed the test.
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

	// hammerConfigsMap contains information about pid, vid, and usbPath values of a
	// detachable base for different models. This information was derived from manual
	// testing and the following hammer file:
	// https://chromium.googlesource.com/chromiumos/platform/ec/+/HEAD/board/hammer/variants.h
	hammerConfigsMap := map[string]hammerRequiredVariables{
		"nocturne":      {pid: "20528", vid: "6353", usbPath: "1-7"},
		"soraka":        {pid: "20523", vid: "6353", usbPath: "1-2"},
		"krane":         {pid: "20540", vid: "6353", usbPath: "1-1.1"},
		"kakadu":        {pid: "20548", vid: "6353", usbPath: "1-1.1"},
		"katsu":         {pid: "20560", vid: "6353", usbPath: "1-1.1"},
		"homestar":      {pid: "20562", vid: "6353", usbPath: "1-1.1"},
		"wormdingler":   {pid: "20567", vid: "6353", usbPath: "1-1.3"},
		"quackingstick": {pid: "20571", vid: "6353", usbPath: "1-1.1"},
	}
	var hammerConfigs hammerRequiredVariables
	assignConfigs := func() error {
		s.Log("Attempting detachable base attributes from the hammer file")
		modelName, err := h.Reporter.Model(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the dut's model")
		}
		hammerConfigs.pid = hammerConfigsMap[modelName].pid
		hammerConfigs.vid = hammerConfigsMap[modelName].vid
		hammerConfigs.usbPath = hammerConfigsMap[modelName].usbPath
		return nil
	}
	utilServiceClient := fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	crosCfgRes, err := utilServiceClient.GetDetachableBaseValue(ctx, &empty.Empty{})
	if err != nil {
		s.Log("Failed to get detachable-base attribute values: ", err)
		if err := assignConfigs(); err != nil {
			s.Fatal("Unable to set attributes: ", err)
		}
	} else {
		hammerConfigs.pid = crosCfgRes.ProductId
		hammerConfigs.vid = crosCfgRes.VendorId
		hammerConfigs.usbPath = crosCfgRes.UsbPath
	}

	tempDir, err := ioutil.TempDir("", "BaseECUpdate")
	if err != nil {
		s.Fatal("Failed to create a temp dir")
	}
	defer os.RemoveAll(tempDir)

	// fileDir creates paths to save the modified base ec bin file at respective locations.
	fileDir := modifiedFileDir{
		onLocal: filepath.Join(tempDir, "modifiedBaseECLocal.bin"),
		onHost:  filepath.Join(tempDir, "modifiedBaseECHost.bin"),
	}

	s.Log("Saving the base ec firmware version before flashing an old image")
	originalBaseEC, err := getBaseECInfo(ctx, dut, hammerConfigs.pid)
	if err != nil {
		s.Fatal("Failed to check base ec's version: ", err)
	}
	s.Log("Flash protection flags: ", originalBaseEC.protectionFlag)

	if err := modifyBaseEC(ctx, dut, originalBaseEC, &fileDir); err != nil {
		s.Fatal("Failed to modify base-ec: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	// In case the detachable base did not update successfully, warm reset DUT
	// at the end of the test to ensure that the base ec would be restored.
	requiredReboot := true
	defer func(ctx context.Context, reboot *bool) {
		if *reboot {
			// One of the previous steps in flashing base ec was probably
			// unsuccessful, which might leave base ec unresponsive.
			s.Log("Rebooting DUT to recover base ec")
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
		}
	}(cleanupCtx, &requiredReboot)

	s.Log("Flashing an old image to detachable-base ec")
	if err := flashAnOldImgToDetachableBaseEC(ctx, dut, hammerConfigs, fileDir.onHost); err != nil {
		// If flashing an edited image fails, check whether the version
		// has changed. Sometimes, this failure might relate to the protection
		// pipeline designed to guarantee a file's integrity, like so:
		// Error message: libminijail[9206]: child process 9207 exited
		// with status 14.
		s.Log("Failed to flash base ec to an old version: ", err)
		flashedBaseEC, err := getBaseECInfo(ctx, dut, hammerConfigs.pid)
		if err != nil {
			s.Fatal("Failed to get base ec info after flash: ", err)
		}

		s.Logf("Base ec version: %q [Before] v.s. %q [After]", originalBaseEC.version, flashedBaseEC.version)
		if baseECVersionUnchanged(originalBaseEC.version[len(originalBaseEC.name)+1:], flashedBaseEC.version[len(flashedBaseEC.name)+1:]) {
			s.Fatalf("Found base ec version unchanged, got before: %q, and after: %q", originalBaseEC.version, flashedBaseEC.version)
		}

	}

	// Given that DUT's base ec is running an old firmware,
	// detaching then re-attaching base would trigger an update
	// notification window to pop up in a logged in session.
	if err := triggerAndFindNotification(ctx, ecTool, utilServiceClient, dut, hammerConfigs.pid); err != nil {
		currentBaseEC, errBaseEC := getBaseECInfo(ctx, dut, hammerConfigs.pid)
		if errBaseEC != nil {
			s.Fatal("Failed to trigger and find notification window, and while getting base ec info: ", errBaseEC)
		}
		s.Fatalf("Failed to trigger and find notification window [current base ec version: %s, ro protected: %t]: %v", currentBaseEC.version, currentBaseEC.roProtected, err)
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

	requiredReboot = false

	s.Log("Saving the base ec firmware version after reboot ")
	newBaseEC, err := getBaseECInfo(ctx, dut, crosCfgRes.ProductId)
	if err != nil {
		s.Fatal("Failed to check base ec's version: ", err)
	}

	s.Log("Verifying that base ec restored after a reboot")
	if !baseECVersionUnchanged(originalBaseEC.version[len(originalBaseEC.name)+1:], newBaseEC.version[len(newBaseEC.name)+1:]) {
		s.Fatal("Failed to update the base ec back to default version")
	}
}

func flashAnOldImgToDetachableBaseEC(ctx context.Context, dut *dut.DUT, hammerConfigs hammerRequiredVariables, dstImg string) error {
	if err := dut.Conn().CommandContext(
		ctx,
		"/sbin/minijail0", "-e", "-N", "-p", "-l", "-u",
		"hammerd", "-g", "hammerd", "-c", "0002", "/usr/bin/hammerd",
		"--ec_image_path="+dstImg,
		"--product_id="+hammerConfigs.pid,
		"--vendor_id="+hammerConfigs.vid,
		"--usb_path="+hammerConfigs.usbPath,
		"--update_if=always",
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to run the hammerd command")
	}
	return nil
}

func baseECVersionUnchanged(old, new string) bool {
	return semver.Compare(old, new) == 0
}

func getBaseECInfo(ctx context.Context, dut *dut.DUT, productIDDecimal string) (baseECInfo, error) {
	var baseEC baseECInfo
	productID, _ := strconv.Atoi(productIDDecimal)
	hexProductID := strconv.FormatInt(int64(productID), 16)
	deviceParams := fmt.Sprintf("18d1:%s", hexProductID)

	outputUsbUpdater := ""
	// Poll on the usb_updater2 command, as the first few iterations
	// might run into the 'can't find device' error.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		output, err := dut.Conn().CommandContext(ctx, "usb_updater2", "-d", deviceParams, "-f").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to run usb_updater2 command in the dut")
		}
		outputUsbUpdater = string(output)
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
		return baseEC, errors.Wrap(err, "failed to get the info from usb_updater2")
	}

	baseECInfoMap := map[string]*regexp.Regexp{
		"name":             regexp.MustCompile(`version:\s+(\w+)_v`),
		"version":          regexp.MustCompile(`version:\s+(\w+.\w.\w+-\w+)`),
		"protection flags": regexp.MustCompile(`Flash protection status:\s+(\w+)`),
	}

	for k, v := range baseECInfoMap {
		match := v.FindStringSubmatch(outputUsbUpdater)
		if len(match) < 2 {
			return baseEC, errors.Errorf("did not match regex %q in %q", v, outputUsbUpdater)
		}
		usbUpdater2Info := strings.TrimSpace(match[1])

		switch k {
		case "name":
			baseEC.name = usbUpdater2Info
		case "version":
			baseEC.version = usbUpdater2Info
		case "protection flags":
			baseEC.protectionFlag = usbUpdater2Info
		}
	}

	flagInDecimal, err := strconv.ParseInt(baseEC.protectionFlag, 16, 64)
	if err != nil {
		return baseEC, errors.Wrap(err, "failed to convert protection flags into hexadecimal")
	}
	flagInBinary := strconv.FormatInt(flagInDecimal, 2)
	// If the second bit is equal to 1, RO is protected now.
	if len(flagInBinary) > 1 && string(flagInBinary[len(flagInBinary)-2]) == "1" {
		baseEC.roProtected = true
	}

	return baseEC, nil
}

func triggerAndFindNotification(ctx context.Context, ecTool *firmware.ECTool, utilSvcClient fwpb.UtilsServiceClient, dut *dut.DUT, hammerPid string) error {

	// Included in baseGpioNames are a list of possible gpios available for
	// controlling the base state. The first one found from the list would
	// be used in setting base state attached/detached.
	var baseStateGpio string
	baseGpioNames := []firmware.GpioName{firmware.ENBASE, firmware.ENPP3300POGO, firmware.PP3300DXBASE}
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
		basestate    string
		value        string
		baseAttached bool
	}{
		{
			basestate:    baseStateGpio,
			value:        "0",
			baseAttached: false,
		},
		{
			basestate:    baseStateGpio,
			value:        "1",
			baseAttached: true,
		},
	} {
		if err := ecTool.Command(ctx, "gpioset", step.basestate, step.value).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to switch the basestate")
		}

		// Allow some delay to ensure base attached/detached by setting the gpio.
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep for 10 seconds for the command to fully propagate to the DUT")
		}

		lsusbInfo, err := dut.Conn().CommandContext(ctx, "lsusb").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to get lsusb info")
		}

		switch step.baseAttached {
		case true:
			if !strings.Contains(string(lsusbInfo), "Hammer") {
				return errors.New("expected keyboard attached, but did not find name 'hammer' from lsusb")
			}
		case false:
			if strings.Contains(string(lsusbInfo), "Hammer") {
				return errors.New("expected keyboard detached, but found name 'hammer' from lsusb")
			}
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

// modifyBaseEC copies the /lib/firmware/base-ec.fw to local,
// modifies its version -1 and puts it back to /tmp/ folder in DUT.
func modifyBaseEC(ctx context.Context, dut *dut.DUT, boardInfo baseECInfo, fileDir *modifiedFileDir) error {

	originalBaseECBinFile := fmt.Sprintf("/lib/firmware/%s.fw", boardInfo.name)

	testing.ContextLog(ctx, "Copying base-ec.fw from DUT to local")
	if err := linuxssh.GetFile(ctx, dut.Conn(), originalBaseECBinFile, fileDir.onLocal, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed to copy base-ec.fw to local")
	}

	f, err := os.Open(fileDir.onLocal)
	if err != nil {
		return errors.Wrap(err, "failed to open base-ec.bin")
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to read base-ec.bin")
	}

	reader := bufio.NewReader(f)
	buf := make([]byte, stat.Size())

	for {
		_, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				testing.ContextLog(ctx, "Unexpected error: ", err)
			}
			break
		}

	}
	testing.ContextLog(ctx, "Current base-ec version: ", boardInfo.version)
	testing.ContextLog(ctx, "Starting to modify base-ec.bin")

	indexRWBoard := len(buf)
	count := bytes.Count(buf, []byte(boardInfo.name+"_v"))
	if count == 0 {
		return errors.Wrapf(err, "did not find %s in the base-ec.bin", boardInfo.name+"_v")
	}

	for i := 0; i < count; i++ {
		indexRWBoard = bytes.LastIndex(buf[:indexRWBoard], []byte(boardInfo.name+"_v"))
		indexVersionToModify := indexRWBoard
		indexVersionToModify += (len(boardInfo.name) + 2)
		// version -1
		buf[indexVersionToModify] = buf[indexVersionToModify] - 1
	}
	testing.ContextLog(ctx, "Modified base-ec version: ", string(buf[indexRWBoard:indexRWBoard+len(boardInfo.version)]))

	// create a new bin file.
	file, err := os.Create(fileDir.onLocal)
	if err != nil {
		return errors.New("failed to create a new bin file")
	}
	defer func() {
		os.Remove(fileDir.onLocal)
		file.Close()
	}()

	if _, err := file.Write(buf); err != nil {
		return errors.New("failed to write modified binary code into a new file")
	}

	testing.ContextLog(ctx, "Copy the modified base-ec.bin back to DUT")
	if _, err := linuxssh.PutFiles(ctx, dut.Conn(), map[string]string{fileDir.onLocal: fileDir.onHost}, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed to copy files into DUT")
	}
	return nil
}
