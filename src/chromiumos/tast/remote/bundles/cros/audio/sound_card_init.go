// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/audio/internal"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// soundCardInitTestParams holds the test parameters.
type soundCardInitTestParams struct {
	SoundCardID string
	AmpCount    uint
	Config      string
	Amp         string
	// List of RDC range per channel.
	RDCRange []rdcRange
}

// soundCardInitTest holds the function signature of the sub test.
type soundCardInitTest struct {
	Func func(context.Context, *rpcdut.RPCDUT, soundCardInitTestParams) error
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SoundCardInit,
		Desc:         "Verifies sound_card_init finishes successfully at boot time",
		SoftwareDeps: []string{"reboot"},
		// Skips atlas, nocturne, lindar, lillipup as they don't use sound_card_init to initialized their smart amps.
		// Skip volteer2 as it's a reference design device not an official launched device.
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2", "lindar", "lillipup", "helios")),
		Contacts:     []string{"judyhsiao@chromium.org", "yuhsuan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "run_success",
				Val: soundCardInitTest{
					Func: runSuccess,
				},
			},
			{
				Name:      "validate_rdc_range",
				ExtraAttr: []string{"informational"},
				Val: soundCardInitTest{
					Func: validateRDCRange,
				},
			},
		},
	})
}

const (
	soundCardInitTimeout = time.Minute

	// soundCardInitRunTimeFile is the file stores previous sound_card_init run time.
	soundCardInitRunTimeFile = "/var/lib/sound_card_init/%s/run"

	// calibrationFiles is the file stores previous calibration values.
	calibrationFiles = "/var/lib/sound_card_init/%s/calib_%d"

	// calibYAMLContent is the content of calibration file.
	calibYAMLContent = `
---
UseVPD
`

	// vpdFiles is the file that stores VPD values.
	vpdFiles = "/sys/firmware/vpd/ro/dsm_calib_r0_%d"
)

// deviceSettings is the sound_card_init config.
type deviceSettings struct {
	AmpCalibrations ampCalibSettings `yaml:"amp_calibrations"`
}

// ampCalibSettings is the amp config section for deviceSettings.
type ampCalibSettings struct {
	RDCRange []rdcRange `yaml:"rdc_ranges"`
}

// rdcRange specified the valid range of rdc (DC resistance) in ohm.
type rdcRange struct {
	LowerBound float32 `yaml:"lower"`
	UpperBound float32 `yaml:"upper"`
}

// rdcInfo store the channel number and the RDC applied to the channel.
type rdcInfo struct {
	Channel  int32   `json:"channel"`
	RDCInOhm float32 `json:"rdc_in_ohm"`
}

// SoundCardInit verifies sound_card_init finishes successfully at boot time.
func SoundCardInit(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	// Ensure the rpc connection is closed at the end of this test.
	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(cleanupCtx)

	dump, err := d.Conn().CommandContext(ctx, "aplay", "-l").Output()
	if err != nil {
		s.Fatal("Failed to aplay -l: ", err)
	}
	soundCardID, err := parseSoundCardID(string(dump))
	if err != nil {
		s.Fatal("Failed to parse sound card name: ", err)
	}

	ampInByte, err := d.Conn().CommandContext(ctx, "cros_config", "/audio/main", "speaker-amp").Output()
	if err != nil {
		s.Fatal("cros_config /audio/main speaker-amp failed: ", err)
	}
	amp := string(ampInByte)
	s.Logf("Amp: %s", amp)

	configInByte, err := d.Conn().CommandContext(ctx, "cros_config", "/audio/main", "sound-card-init-conf").Output()
	if err != nil {
		s.Fatal("cros_config /audio/main sound-card-init-conf failed: ", err)
	}
	config := string(configInByte)
	s.Logf("sound_card_init config: %s", config)

	rdcRange, err := rdcRanges(ctx, d, config)
	if err != nil {
		s.Fatal("Failed to get rdc range: ", err)
	}
	if len(rdcRange) == 0 {
		s.Fatal("rdcRange is empty")
	}
	numCh := uint(len(rdcRange))

	if err := verifyVPDExist(ctx, d, numCh); err != nil {
		s.Fatal("Missing VPD: ", err)
	}

	if err := removeSoundCardInitFiles(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to remove previous files: ", err)
	}
	if err := createCalibrationFiles(ctx, d, soundCardID, numCh); err != nil {
		s.Fatal("Failed to create calibration files: ", err)
	}

	defer func(ctx context.Context) {
		// Clean up calibration files.
		if err := removeCalibrationFiles(ctx, d, soundCardID, numCh); err != nil {
			s.Fatal("Failed to clean up calibration files: ", err)
		}
	}(cleanupCtx)

	s.Log("Reboot the device")
	// Reboot
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	param := soundCardInitTestParams{
		Amp:         amp,
		SoundCardID: soundCardID,
		Config:      config,
		AmpCount:    numCh,
		RDCRange:    rdcRange,
	}

	// Run test cases.
	testFunc := s.Param().(soundCardInitTest).Func
	s.Logf("Running test %q", s.TestName())
	if err := testFunc(ctx, d, param); err != nil {
		s.Fatalf("%s test failed: %v", s.TestName(), err)
	}
}

func parseSoundCardID(dump string) (string, error) {
	re := regexp.MustCompile(`card 0: ([a-z0-9]+) `)
	m := re.FindStringSubmatch(dump)

	if len(m) != 2 {
		return "", errors.New("no sound card")
	}
	return m[1], nil
}

// removeSoundCardInitFiles removes all sound_card_init files.
func removeSoundCardInitFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	if err := fs.Remove(ctx, internal.CrasStopTimeFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to rm file: %s", internal.CrasStopTimeFile)
	}
	file := fmt.Sprintf(soundCardInitRunTimeFile, soundCardID)
	if err := fs.Remove(ctx, file); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to rm file: %s", file)
	}
	return nil
}

// createCalibrationFiles creates the calibration files on DUT.
func createCalibrationFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string, count uint) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	for i := 0; i < int(count); i++ {
		f := fmt.Sprintf(calibrationFiles, soundCardID, i)
		exists, err := fs.Exists(ctx, f)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", f)
		}
		if !exists {
			if err := fs.WriteFile(ctx, f, []byte(calibYAMLContent), 0644); err != nil {
				return errors.Wrapf(err, "failed to create %s", f)
			}
		}
	}
	return nil
}

// removeCalibrationFiles removes the calibration files on DUT.
func removeCalibrationFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string, count uint) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	for i := 0; i < int(count); i++ {
		file := fmt.Sprintf(calibrationFiles, soundCardID, i)
		if err := fs.Remove(ctx, file); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to rm file: %s", file)
		}
	}
	return nil
}

// verifyVPDExist returns error if VPD files do not exist.
func verifyVPDExist(ctx context.Context, d *rpcdut.RPCDUT, count uint) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	for i := 0; i < int(count); i++ {
		f := fmt.Sprintf(vpdFiles, i)
		exists, err := fs.Exists(ctx, f)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", f)
		}
		if !exists {
			return errors.New(f + " does not exist")
		}
	}
	return nil
}

// verifySoundCardInitFinished polls for sound_card_init run time file being updated, which means sound_card_init completes running.
func verifySoundCardInitFinished(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		file := fmt.Sprintf(soundCardInitRunTimeFile, soundCardID)
		exists, err := fs.Exists(ctx, file)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", file)
		}
		if exists {
			return nil
		}
		return errors.New(file + " does not exist")
	}, &testing.PollOptions{Timeout: soundCardInitTimeout})
	return err
}

func rdcRanges(ctx context.Context, d *rpcdut.RPCDUT, config string) ([]rdcRange, error) {
	fs := dutfs.NewClient(d.RPC().Conn)
	yamlPath := fmt.Sprintf("/etc/sound_card_init/%s", config)
	yamlFile, err := fs.ReadFile(ctx, yamlPath)
	if err != nil {
		return nil, err
	}
	conf := &deviceSettings{}
	err = yaml.Unmarshal(yamlFile, conf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", yamlPath)
	}
	return conf.AmpCalibrations.RDCRange, nil
}

func appliedRDC(ctx context.Context, d *rpcdut.RPCDUT, soundCardID, config, amp string, ch int) (*rdcInfo, error) {
	// Run sound_card_init.
	dump, err := d.Conn().CommandContext(
		ctx,
		"/usr/bin/sound_card_init",
		"--id="+soundCardID,
		"--conf="+config,
		"--amp="+amp,
		"--read_applied_rdc="+strconv.Itoa(ch),
	).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get applied rdc")
	}

	var rdc rdcInfo
	if err := json.Unmarshal(dump, &rdc); err != nil {
		return nil, errors.New("failed to parse applied rdc: " + string(dump))
	}
	return &rdc, nil
}

// runSuccess verifies sound_card_init completes running without any error.
func runSuccess(ctx context.Context, d *rpcdut.RPCDUT, param soundCardInitTestParams) error {
	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, param.SoundCardID); err != nil {
		return errors.Wrap(err, "failed to wait for sound_card_init completion")
	}
	return nil
}

// validateRDCRange verifies the applied rdc is withing a valid range.
func validateRDCRange(ctx context.Context, d *rpcdut.RPCDUT, param soundCardInitTestParams) error {
	numCh := param.AmpCount
	config := param.Config
	soundCardID := param.SoundCardID
	amp := param.Amp
	rdcRange := param.RDCRange

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, soundCardID); err != nil {
		return errors.Wrap(err, "failed to wait for sound_card_init completion")
	}

	for i := 0; i < int(numCh); i++ {
		rdc, err := appliedRDC(ctx, d, soundCardID, config, amp, i)
		if err != nil {
			return errors.Wrapf(err, "failed to get applied rdc%d", i)
		}
		if rdc.RDCInOhm < rdcRange[i].LowerBound || rdc.RDCInOhm > rdcRange[i].UpperBound {
			return errors.Errorf("invalid rdc_%d: %f, want:[%f, %f]", rdc.Channel, rdc.RDCInOhm, rdcRange[i].LowerBound, rdcRange[i].UpperBound)
		}
	}

	return nil
}
