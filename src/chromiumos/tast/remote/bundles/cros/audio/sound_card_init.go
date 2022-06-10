// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
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

func init() {
	testing.AddTest(&testing.Test{
		Func: SoundCardInit,
		Desc: "Verifies sound_card_init finishes successfully at boot time",
		// Skips atlas, nocturne, lindar, lillipup as they don't use sound_card_init to initialized their smart amps.
		// Skip volteer2 as it's a reference design device not an official launched device.
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2", "lindar", "lillipup", "helios")),
		Contacts:     []string{"judyhsiao@chromium.org", "yuhsuan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		// TODO(b/198550559) : remove "no_manatee" when b/198550559 is fixed.
		SoftwareDeps: []string{"no_manatee"},
		Timeout:      5 * time.Minute,
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
	AmpCalibrations AmpCalibSettings `yaml:"amp_calibrations"`
}

// ampCalibSettings is the amp config section for DeviceSettings.
type ampCalibSettings struct {
	RdcRange []float32 `yaml:"rdc_range"`
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

func getRdcRange(ctx context.Context, d *rpcdut.RPCDUT, config string) ([]float32, error) {
	fs := dutfs.NewClient(d.RPC().Conn)
	yamlPath := fmt.Sprintf("/etc/sound_card_init/%s", config)
	yamlFile, err := fs.ReadFile(ctx, yamlPath)
	if err != nil {
		return nil, err
	}
	conf := &DeviceSettings{}
	err = yaml.Unmarshal(yamlFile, conf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", yamlPath)
	}
	return conf.AmpCalibrations.RdcRange, nil
}

func getAppliedRdc(ctx context.Context, d *rpcdut.RPCDUT, soundCardID, config, amp string, ch int) (float32, error) {
	// Run sound_card_init.
	dump, err := d.Conn().CommandContext(
		ctx,
		"/usr/bin/sound_card_init",
		"--id="+soundCardID,
		"--conf="+config,
		"--amp="+amp,
		"--read_applied_rdc="+strconv.Itoa(ch),
	).Output()

	re := regexp.MustCompile(`channel: [0-9]+ rdc: ([0-9]*\.[0-9]{3}) ohm`)
	match := re.FindStringSubmatch(string(dump))
	if match == nil {
		return 0, errors.New("failed to match applied rdc")
	}
	if rdc, err := strconv.ParseFloat(match[1], 32); err == nil {
		return float32(rdc), nil
	}
	return 0, errors.Wrap(err, "failed to parse applied rdc")
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
	numCh := uint(len(rdcRange) / 2)

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

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to wait for sound_card_init completion: ", err)
	}

	for i := 0; i < int(numCh); i++ {
		rdc, err := getAppliedRdc(ctx, d, soundCardID, config, amp, i)
		s.Logf("rdc_%d: %f ohm", i, rdc)
		if err != nil {
			s.Fatalf("Failed to get applied rdc%d: %s", i, err)
		}
		if rdc < rdcRange[2*i] || rdc > rdcRange[2*i+1] {
			s.Fatalf("Invalid rdc_ %d: %f, expect:[%f, %f]", i, rdc, rdcRange[2*i], rdcRange[2*i+1])
		}
	}
}
