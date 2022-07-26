// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type fwinfo struct {
	Board string `json:"board_name"`
	Model string `json:"model_name"`
	FwID  string `json:"firmware_build_cros_version"`
}

const (
	// firmwareFileName contains the name of the file to be downloaded from chromeos-image-archive.
	firmwareFileName = "firmware_from_source.tar.bz2"

	// flashingTime sets the timeout for the flashing process.
	flashingTime = 20 * time.Minute

	// reconnectTime sets the timeout to reconnect DUT after flashing.
	reconnectTime = 10 * time.Minute

	// speedometerTime sets the timeout for Speedometer test.
	speedometerTime = 10 * time.Minute

	// deviationTarget contains the acceptable percentage of deviation from the baseline.
	deviationTarget = 0.05
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APROBootabilityPerformance,
		Desc:         "Ensure bootability and system level performance with old RO AP builds",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_trial"},
		Vars:         []string{"board", "model"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      60 * time.Minute, // 1hr.
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.NormalMode,
		Data:         []string{"shipped-firmwares.json"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Model("vilboz", "dirinboz", "sparky360", "apel", "delbin")), // Temporarily constraining the test to a few models.
	})
}

func APROBootabilityPerformance(ctx context.Context, s *testing.State) {
	/* The overall logic carried out by the test is:
	RO_old   + RW_old - this will be the baseline
	RO_old   + RW_new - compare it to baseline
	RO_old-1 + RW_new - compare it to baseline
	RO_old-2 + RW_new - compare it to baseline
	.
	.
	RO_old-n + RW_new - compare it to baseline
	*/
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// Confirm the CCD is open.
	if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
		s.Fatal("Failed while checking if servo has a CCD connection: ", err)
	} else if hasCCD {
		if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
			s.Fatal("Failed to get gsc_ccd_level: ", err)
		} else if val != servo.Open {
			s.Logf("CCD is not open, got %q. Attempting to unlock", val)
			if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
				s.Fatal("Failed to unlock CCD: ", err)
			}
		}
	}

	s.Log("Disabling hardware write protect")
	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		s.Fatal("Failed to disable hardware write protect: ", err)
	}

	s.Log("Disabling EC software write protect")
	if err := h.Servo.RunECCommand(ctx, "flashwp disable now"); err != nil {
		s.Fatal("Failed to disable EC software write protect: ", err)
	}

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to open RPC client: ", err)
	}

	s.Log("Disabling AP software write protect")
	bs := fwpb.NewBiosServiceClient(h.RPCClient.Conn)
	if _, err := bs.SetAPSoftwareWriteProtect(ctx, &fwpb.WPRequest{Enable: false}); err != nil {
		s.Fatal("Failed to disable AP software write protection: ", err)
	}

	// The 'SHIPPED' firmware IDs can be generated and exported to a json file
	// by running the following bq command:
	/*
		bq query --use_legacy_sql=false --format json -n 3000 'SELECT DISTINCT id, board_name, model_name, firmware_build_cros_version
		FROM `google.com:cros-goldeneye.prod.FirmwareQuals`
		WHERE ship_status="SHIPPED" ORDER BY board_name, model_name, id' > shipped-firmwares.json
	*/
	// The json file was manually deposited as internal data under 'firmware/data'.

	// Read from the 'shipped-firmwares.json' file.
	filepath := s.DataPath("shipped-firmwares.json")
	shippedFwVersions, err := collectShippedFws(h, filepath)
	if err != nil {
		s.Fatal("While collecting the shipped fw versions: ", err)
	}
	s.Logf("SHIPPED firmwares found for model %s: %s", h.Model, shippedFwVersions)

	// Create a new directory to store the downloaded files.
	tmpDir, err := ioutil.TempDir("", "firmware-APROBootabilityPerformance")
	if err != nil {
		s.Fatal("Failed to create a new directory for the test: ", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download the latest shipped firmware.
	board := firmware.CfgPlatformFromLSBBoard(h.Board)
	if err := downloadFirmwareFile(ctx, s, board, shippedFwVersions[len(shippedFwVersions)-1], tmpDir); err != nil {
		s.Fatal("Failed while downloading file: ", err)
	}

	// Get the model name from 'crossystem fwid'
	rwfwid, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamFwid)
	if err != nil {
		s.Fatal("Failed to get crossystem fwid: ", err)
	}
	splitout := strings.Split(rwfwid, ".")
	splitout = strings.Split(splitout[0], "_")
	fwidModel := strings.ToLower(splitout[1])

	// Untar the binary file with respect to the model name found in 'crossystem fwid'.
	filename, err := untarUnknownFileName(ctx, tmpDir, fwidModel)
	if err != nil {
		s.Fatal("Failed to untar file: ", err)
	}

	// Get the current version ID of RW firmware that's running on DUT.
	// This version would be the to-be-qualified RW_new firmware.
	rwNewID, err := getOnlyID(ctx, h, reporters.CrossystemParamFwid)
	if err != nil {
		s.Fatal("Failed to get current RW version number: ", err)
	}

	// Create a copy of the RW_new firmware.
	s.Log("Backing up AP firmware")
	newRWfwFile, err := bs.BackupImageSection(ctx, &fwpb.FWSectionInfo{Section: fwpb.ImageSection_EmptyImageSection, Programmer: fwpb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current AP firmware: ", err)
	}

	// Store the backup file in a temporary directory with the name 'newRW'.
	s.Log("Saving the AP firmware")
	if err := linuxssh.GetFile(ctx, s.DUT().Conn(), newRWfwFile.Path, tmpDir+"/newRW", linuxssh.DereferenceSymlinks); err != nil {
		s.Log("Failed to save the new RW firmware")
	}

	// At the end of this test, restore AP firmware to the one found at the beginning.
	defer func() {
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to get bios service: ", err)
		}

		// Flashing RW_new firmware obtained from the DUT at the beginning of the test.
		s.Log("Restoring AP firmware at the end of the test")
		if err := flashDUTAndReboot(ctx, h, s.DUT().Conn(), "newRW", tmpDir, fwpb.ImageSection_EmptyImageSection); err != nil {
			s.Fatal("Failed to flash DUT: ", err)
		}
	}()

	// Flash the latest shipped RO and RW firmware.
	if err := flashDUTAndReboot(ctx, h, s.DUT().Conn(), filename, tmpDir, fwpb.ImageSection_EmptyImageSection); err != nil {
		s.Fatal("Failed to flash DUT: ", err)
	}

	// Verify RO/RW firmware versions are the latest shipped firmware after flashing.
	// This is when RO and RW have the same version ids (i.e., RO_old + RW_old).
	if err = verifyFwIDs(ctx, h, shippedFwVersions[len(shippedFwVersions)-1], shippedFwVersions[len(shippedFwVersions)-1]); err != nil {
		s.Fatal("While comparing firmware versions: ", err)
	}

	s.Log("Performing the speed test")
	baseline, err := speedTest(ctx, h)
	if err != nil {
		s.Fatal("Failed to perform Speedometer test: ", err)
	}
	s.Logf("Setting the baseline as: %f", baseline)

	// Skip speedometer test if the RW_new firmware is the same as the
	// RO_old shipped version because this was already verified and set as baseline.
	if shippedFwVersions[len(shippedFwVersions)-1] == rwNewID {
		s.Log("WARNING! Speed test skipped because RW_new is the same as RO_old. Already verified")
	} else {
		// Setting DUT to boot from RW section A in the next reboot.
		// This will assure that the DUT will try to boot from the flashed section.
		if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fw_try_next=A").Run(); err != nil {
			s.Fatal("Failed to set 'crossystem fw_try_next=A': ", err)
		}

		// Flashing RW_new firmware obtained from the DUT at the beginning of the test into RW section A.
		// This will leave the DUT with the latest RO shipped fw and
		// the to-be-qualified new RW firmware (i.e., RO_old + RW_new).
		s.Log("Flashing the to-be-qualified new RW firmware")
		if err := flashDUTAndReboot(ctx, h, s.DUT().Conn(), "newRW", tmpDir, fwpb.ImageSection_APRWAImageSection); err != nil {
			s.Fatal("Failed to flash DUT: ", err)
		}

		// Verify that the RO firmware has not been modified and RW has the RW_new after the flashing process.
		if err := verifyFwIDs(ctx, h, shippedFwVersions[len(shippedFwVersions)-1], rwNewID); err != nil {
			s.Fatal("While comparing firmware versions: ", err)
		}

		s.Log("Performing the speed test")
		speedResult, err := speedTest(ctx, h)
		if err != nil {
			s.Fatal("Failed to perform Speedometer test: ", err)
		}

		// Check that the result deviation from the baseline is acceptable.
		if err = checkDeviation(ctx, baseline, speedResult); err != nil {
			s.Fatal("Deviation failed: ", err)
		}
	}

	// Repeat steps for older RO firmware versions (i.e., RO_old-n + RW_new).
	for i := len(shippedFwVersions) - 2; i >= 0; i-- {
		s.Log("Downloading an older shipped firmware file")
		if err := downloadFirmwareFile(ctx, s, board, shippedFwVersions[i], tmpDir); err != nil {
			s.Fatal("Failed while downloading file: ", err)
		}

		s.Log("Untaring file")
		if err := testexec.CommandContext(ctx, "tar", "-xvf", tmpDir+"/"+firmwareFileName, "-C", tmpDir, filename).Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to untar file: ", err)
		}

		s.Log("Flashing the older RO 'shipped' firmware")
		if err := flashDUTAndReboot(ctx, h, s.DUT().Conn(), filename, tmpDir, fwpb.ImageSection_APROImageSection); err != nil {
			s.Fatal("Failed to flash DUT: ", err)
		}

		s.Log("Verifying the firmware versions after flash")
		if err := verifyFwIDs(ctx, h, shippedFwVersions[i], rwNewID); err != nil {
			s.Fatal("Failed while checking firmware versions: ", err)
		}

		s.Log("Performing the speed test")
		speedResult, err := speedTest(ctx, h)
		if err != nil {
			s.Fatal("Failed to perform Speedometer test: ", err)
		}

		s.Log("Checking that the result deviation from the baseline is acceptable")
		if err = checkDeviation(ctx, baseline, speedResult); err != nil {
			s.Fatal("Deviation failed: ", err)
		}
	}

}

// downloadFirmwareFile will download a tar file from cloud and save to a temporary directory,
// based on the shipped firmware version passed in for test.
func downloadFirmwareFile(ctx context.Context, s *testing.State, board, fwid, tmpDir string) error {
	// Regular expression for the path to the image archive.
	re := regexp.MustCompile(`gs:\/\/chromeos-image-archive\/` + board + `-firmware\/[R].*-` + fwid)

	// Get the complete path to the file.
	var path string
	dir := "gs://chromeos-image-archive/" + board + "-firmware/"
	out, err := testexec.CommandContext(ctx, "gsutil", "ls", dir).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run 'gsutil ls' to find the complete path")
	}
	path = re.FindString(string(out))
	if path == "" {
		return errors.Errorf("no image archive found for firmware id: %s board: %s", fwid, board)
	}

	// Stage the complete path.
	cs := s.CloudStorage()
	url := path + "/" + firmwareFileName
	r, err := cs.Stage(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to stage file")
	}

	// Download the file.
	if err := testexec.CommandContext(ctx, "wget", "-P", tmpDir, r.String()).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to download the file")
	}

	// Rename the file.
	if err := testexec.CommandContext(ctx, "mv", tmpDir+"/firmware_from_source.tar.bz2?gs_bucket=chromeos-image-archive", tmpDir+"/"+firmwareFileName).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to rename the file")
	}

	return nil
}

// flashDUTAndReboot will send the bin files to a directory in the DUT, flash the files into the DUT with the bios service 'WriteImageFromMultiSectionFile'
// and reboot the DUT so that the flash takes effect.
func flashDUTAndReboot(ctx context.Context, h *firmware.Helper, conn *ssh.Conn, fwid, tmpDir string, section fwpb.ImageSection) error {
	flashingCtx, cancelflashingCtx := context.WithTimeout(ctx, flashingTime)
	defer cancelflashingCtx()

	testing.ContextLog(flashingCtx, "Sending firmware bin file to DUT")
	if _, err := linuxssh.PutFiles(flashingCtx, conn, map[string]string{tmpDir + "/" + fwid: "/tmp/" + fwid}, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed to send bin file to DUT")
	}

	testing.ContextLogf(ctx, "Flashing DUT with file: %s using section: %v", fwid, section)
	bs := fwpb.NewBiosServiceClient(h.RPCClient.Conn)
	if _, err := bs.WriteImageFromMultiSectionFile(ctx, &fwpb.FWSectionInfo{Programmer: fwpb.Programmer_BIOSProgrammer, Path: "/tmp/" + fwid, Section: section}); err != nil {
		return errors.Wrap(err, "failed to flash DUT with the multi-section bin file")
	}

	// Close RPC connection before reboot.
	h.CloseRPCConnection(ctx)

	// Reboot DUT for flash to take effect.
	testing.ContextLog(ctx, "Power-cycling DUT with a warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		return errors.Wrap(err, "failed to reboot DUT by servo")
	}

	testing.ContextLog(ctx, "Waiting for DUT to reconnect")
	connectCtx, cancelconnectCtx := context.WithTimeout(ctx, reconnectTime)
	defer cancelconnectCtx()

	if err := h.WaitConnect(connectCtx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}

	// Open RPC connection after reboot.
	if err := h.RequireRPCClient(ctx); err != nil {
		return errors.Wrap(err, "failed to open RPC client after reboot")
	}

	return nil
}

// verifyFwIDs will show in logs the current firmware version and compare it to expected ones if they are provided.
func verifyFwIDs(ctx context.Context, h *firmware.Helper, exVersion ...string) error {
	sections := []reporters.CrossystemParam{reporters.CrossystemParamRoFwid, reporters.CrossystemParamFwid}
	for i := range sections {
		currentID, err := getOnlyID(ctx, h, sections[i])
		if err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Current %s: %s", sections[i], currentID)
		if i < len(exVersion) {
			if exVersion[i] != currentID {
				return errors.Errorf("got %s=%s, but expected %s", sections[i], currentID, exVersion[i])
			}
		}
	}
	return nil
}

// getOnlyID accepts 'crossystem' params (i.e., CrossystemParamFwid & CrossystemParamRoFwid),
// splits the outputs from them and only returns the version numbers.
func getOnlyID(ctx context.Context, h *firmware.Helper, param reporters.CrossystemParam) (string, error) {
	fwid, err := h.Reporter.CrossystemParam(ctx, param)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get only the fw id from crossystem: %v", param)
	}
	splitout := strings.Split(fwid, ".")
	onlyID := splitout[1] + "." + splitout[2] + "." + splitout[3]
	return onlyID, err
}

// speedTest performs the speedometer2 test.
func speedTest(ctx context.Context, h *firmware.Helper) (float64, error) {
	speedometerCtx, cancelspeedometerCtx := context.WithTimeout(ctx, speedometerTime)
	defer cancelspeedometerCtx()

	testing.ContextLog(ctx, "Sleeping for a few seconds before starting a new Chrome")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return 0.0, errors.Wrap(err, "failed to wait for a few seconds")
	}

	speedometerService := fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	if _, err := speedometerService.NewChrome(speedometerCtx, &empty.Empty{}); err != nil {
		return 0.0, errors.Wrap(err, "failed to initiate a chrome sesion")
	}
	defer func() error {
		if _, err := speedometerService.CloseChrome(speedometerCtx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to close the chrome sesion")
		}
		return nil
	}()

	testing.ContextLog(speedometerCtx, "Running speedometer test")
	sptest, err := speedometerService.PerformSpeedometerTest(speedometerCtx, &empty.Empty{})
	if err != nil {
		return 0.0, errors.Wrap(err, "failed while performing the Speedometer benchmark")
	}

	// Pars the output of the test as a float for later math operations.
	result, err := strconv.ParseFloat(sptest.Result, 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to convert the result into float")
	}

	testing.ContextLogf(speedometerCtx, "Speedometer Result: %f", result)
	return result, nil
}

// checkDeviation will verify if the result is inside the accepted deviation range.
func checkDeviation(ctx context.Context, baseline, result float64) error {
	deviation := (baseline * deviationTarget)
	upperBound := baseline + deviation
	lowerBound := baseline - deviation
	if result > upperBound {
		return errors.Errorf("speedometer result %v is HIGHER than targeted deviation of %v from baseline %v", result, deviationTarget, baseline)
	}
	if result < lowerBound {
		return errors.Errorf("speedometer result %v is LOWER than targeted deviation of %v from baseline %v", result, deviationTarget, baseline)
	}
	testing.ContextLog(ctx, "Result is inside the limits of deviation")
	return nil
}

// collectShippedFws will parse the firmware IDs from the json file.
func collectShippedFws(h *firmware.Helper, filepath string) ([]string, error) {
	out, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read JSON file")
	}

	var data []fwinfo
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, errors.Wrap(err, "failed to parse JSON file")
	}

	var shippedFws []string
	for _, values := range data {
		if values.Model == h.Model {
			shippedFws = append(shippedFws, values.FwID)
		}
	}

	if len(shippedFws) == 0 {
		return nil, errors.Errorf("did not find any shipped fw for %s", h.Model)
	}

	return shippedFws, nil
}

// untarUnknownFileName will try to untar the respective fw bin file from the downloaded tar file.
func untarUnknownFileName(ctx context.Context, tmpDir, fwidModel string) (string, error) {
	filename := "image-" + fwidModel + ".bin"
	testing.ContextLogf(ctx, "Untaring file %q from %s", filename, firmwareFileName)
	if err := testexec.CommandContext(ctx, "tar", "-xvf", tmpDir+"/"+firmwareFileName, "-C", tmpDir, filename).Run(ssh.DumpLogOnError); err != nil {
		// Sometimes the file name format will not be "image-board.bin" but just "image.bin" instead.
		testing.ContextLogf(ctx, "WARNING! failed to untar the image with the name: %q", filename)
		filename = "image.bin"
		testing.ContextLogf(ctx, "Retry with name: %s", filename)
		if err := testexec.CommandContext(ctx, "tar", "-xvf", tmpDir+"/"+firmwareFileName, "-C", tmpDir, filename).Run(ssh.DumpLogOnError); err != nil {
			return "", errors.Wrapf(err, "failed to untar the file with name %q", filename)
		}
	}
	return filename, nil
}
