// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters holds the test parameters.
type subTestParameters struct {
	SoundCardID string
	AmpCount    uint
	Config      string
	Amp         string
	RdcRange    []rdcRange
}

// testParameters holds the test parameters.
type testParameters struct {
	Func func(context.Context, *rpcdut.RPCDUT, subTestParameters) error
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SoundCardInit,
		Desc: "Verifies sound_card_init finishes successfully at boot time",
		// Skips atlas, nocturne, lindar, lillipup as they don't use sound_card_init to initialized their smart amps.
		// Skip volteer2 as it's a reference design device not an official launched device.
		// TODO(b/221702936) : remove "kano" when b/221702936 is fixed.
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2", "lindar", "lillipup", "helios", "kano")),
		Contacts:     []string{"judyhsiao@chromium.org", "yuhsuan@chromium.org"},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "exit_success",
				Val: testParameters{
					Func: exitSuccess,
				},
			},
			{
				Name:      "validate_rdc_range",
				ExtraAttr: []string{"informational"},
				Val: testParameters{
					Func: validateRdcRange,
				},
			},
		},
	})
}

const (
	soundCardInitTimeout = time.Minute

	// soundCardInitRunTimeFile is the file stores previous sound_card_init run time.
	soundCardInitRunTimeFile = "/var/lib/sound_card_init/%s/run"

	// crasStopTimeFile is the file stores previous CRAS stop time.
	crasStopTimeFile = "/var/lib/cras/stop"

	// calibrationFiles is the file stores previous calibration values.
	calibrationFiles = "/var/lib/sound_card_init/%s/calib_%d"

	// calibYAMLContent is the content of calibration file.
	calibYAMLContent = `
---
UseVPD
`
)

// deviceSettings is the sound_card_init config.
type deviceSettings struct {
	AmpCalibrations ampCalibSettings `yaml:"amp_calibrations"`
}

// ampCalibSettings is the amp config section for deviceSettings.
type ampCalibSettings struct {
	RdcRange []rdcRange `yaml:"rdc_ranges"`
}

// rdcRange specified the valid range of rdc in ohm.
type rdcRange struct {
	LowerBound float32 `yaml:"lower"`
	UpperBound float32 `yaml:"upper"`
}

// appliedRDC is the RDC applied to the amp.
type appliedRDC struct {
	Channel  int32   `json:"channel"`
	RdcInOhm float32 `json:"rdc_in_ohm"`
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
	if err := fs.Remove(ctx, crasStopTimeFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to rm file: %s", crasStopTimeFile)
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

func getRdcRange(ctx context.Context, d *rpcdut.RPCDUT, config string) ([]rdcRange, error) {
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
	return conf.AmpCalibrations.RdcRange, nil
}

func getAppliedRdc(ctx context.Context, d *rpcdut.RPCDUT, soundCardID, config, amp string, ch int) (*appliedRDC, error) {
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

	var rdc appliedRDC
	if err := json.Unmarshal(dump, &rdc); err != nil {
		return nil, errors.New("failed to parse applied rdc: " + string(dump))
	}
	return &rdc, nil
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

	configInByte, err := d.Conn().CommandContext(ctx, "cros_config", "/audio/main", "sound-card-init-conf").Output()
	if err != nil {
		s.Fatal("cros_config /audio/main sound-card-init-conf failed: ", err)
	}
	config := string(configInByte)
	s.Log("sound_card_init config: ", config)

	ampInByte, err := d.Conn().CommandContext(ctx, "cros_config", "/audio/main", "speaker-amp").Output()
	if err != nil {
		s.Fatal("cros_config /audio/main speaker-amp failed: ", err)
	}
	amp := string(ampInByte)
	s.Log("Amp: ", amp)

	rdcRange, err := getRdcRange(ctx, d, config)
	if err != nil {
		s.Fatal("Failed to get rdc range: ", err)
	}
	if len(rdcRange) == 0 {
		s.Fatal("rdcRange is empty")
	}
	numCh := uint(len(rdcRange))

	if err := removeSoundCardInitFiles(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to remove previous files: ", err)
	}
	if err := createCalibrationFiles(ctx, d, soundCardID, numCh); err != nil {
		s.Fatal("Failed to create calibration files: ", err)
	}

	defer func() {
		// Clean up calibration files.
		if err := removeCalibrationFiles(ctx, d, soundCardID, numCh); err != nil {
			s.Fatal("Failed to clean up calibration files: ", err)
		}
	}()

	s.Log("Reboot the device")
	// Reboot
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	param := subTestParameters{
		Amp:         amp,
		SoundCardID: soundCardID,
		Config:      config,
		AmpCount:    numCh,
		RdcRange:    rdcRange,
	}

	// Run test cases.
	testFunc := s.Param().(testParameters).Func
	s.Logf("Running test %q", s.TestName())
	if err := testFunc(ctx, d, param); err != nil {
		s.Fatalf("%s test failed: %v", s.TestName(), err)
	}
}

// exitSuccess verifies sound_card_init completes running without any error.
func exitSuccess(ctx context.Context, d *rpcdut.RPCDUT, param subTestParameters) error {
	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, param.SoundCardID); err != nil {
		return errors.Wrap(err, "failed to wait for sound_card_init completion")
	}
	return nil
}

// validateRdcRange verifies the applied rdc is withing a valid range.
func validateRdcRange(ctx context.Context, d *rpcdut.RPCDUT, param subTestParameters) error {
	numCh := param.AmpCount
	config := param.Config
	soundCardID := param.SoundCardID
	amp := param.Amp
	rdcRange := param.RdcRange

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, param.SoundCardID); err != nil {
		return errors.Wrap(err, "failed to wait for sound_card_init completion")
	}

	for i := 0; i < int(numCh); i++ {
		rdc, err := getAppliedRdc(ctx, d, soundCardID, config, amp, i)
		if err != nil {
			return errors.Wrapf(err, "failed to get applied rdc%d", i)
		}
		if rdc.RdcInOhm < rdcRange[i].LowerBound || rdc.RdcInOhm > rdcRange[i].UpperBound {
			return errors.Errorf("invalid rdc_%d: %f, expect:[%f, %f]", rdc.Channel, rdc.RdcInOhm, rdcRange[i].LowerBound, rdcRange[i].UpperBound)
		}
	}

	return nil
}
