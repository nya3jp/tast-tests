// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const soundFile = "open_settings.raw"
const busNumber = 1

var (
	chameleonHostname = testing.RegisterVarString(
		"assistant.chameleon_host",
		"localhost",
		"Hostname for Chameleon")

	chameleonSSHPort = testing.RegisterVarString(
		"assistant.chameleon_ssh_port",
		"22",
		"SSH port for Chameleon")

	chameleonPort = testing.RegisterVarString(
		"assistant.chameleon_port",
		"9992",
		"Port for chameleond on Chameleon")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenSettingsAudioChameleon,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests opening the Settings app using an Assistant query with hotword",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Data:         []string{soundFile},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Vars: []string{
			"assistant.chameleon_host",
			"assistant.chameleon_ssh_port",
			"assistant.chameleon_port",
		},
		Fixture: "assistant",
	})
}

// OpenSettingsAudioChameleon tests that the Settings app can be opened by the Assistant
func OpenSettingsAudioChameleon(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Setup Chameleon
	// In Skylab, DUT and chameleon follow the naming convention: <dut> and <dut>-chameleon
	// While DUT and chamelon can ssh directly though IPs against each other, they cannot
	// resolve machine names to IPs and IP resolution has to be done outside of the local test.
	// Drone keeps the metadata of DUT and chameleon and can help resolve hostname to IP.
	// Drone will pass information like chameleon host, chameleon host_port, ssh_port as
	// tast input through the autotest control file.
	chameleonAddr := fmt.Sprintf("%s:%s", chameleonHostname.Value(), chameleonPort.Value())
	chameleond, err := chameleon.New(ctx, chameleonAddr)
	if err != nil {
		s.Fatal("Failed to connect to chameleon board: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer chameleond.Close(cleanupCtx)

	// reset Chameleon to ensure a consistent state for testing
	chameleond.Reset(ctx)

	if hasAudioSupport, err := chameleond.HasAudioSupport(ctx, chameleon.LineOut); !hasAudioSupport || err != nil {
		s.Fatalf("Chameleon has no audio support for %s: %v", chameleon.LineOut, err)
	}

	// Copy file from DUT to Chameleon
	s.Log("Copy sound file from DUT to Chameleon")
	dstFileName, err := copyFileToHost(ctx, chameleonHostname.Value(), s.DataPath(soundFile), "")
	if err != nil {
		s.Fatal("Failed to copy sound file to chameleon: ", err)
	}
	// Best effort clean up sound file from Chameleon
	defer deleteFileFromHost(ctx, chameleonHostname.Value(), chameleonSSHPort.Value(), dstFileName)

	// Enable DUT to detect audio hotword
	if err := assistant.SetHotwordEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable Hotword in assistant: ", err)
	}

	// connect audio input and output endpoints through bus 1
	chameleond.AudioBoardConnect(ctx, busNumber, chameleon.FPGALineIn)
	chameleond.AudioBoardConnect(ctx, busNumber, chameleon.PeripheralSpeaker)
	defer chameleond.AudioBoardClearRoutes(ctx, busNumber)

	s.Log("Play audio to trigger assistant features")
	if err := chameleond.StartPlayingAudio(ctx, chameleon.LineOut, dstFileName, chameleon.SupportdAudioDataFormat); err != nil {
		s.Fatal("Failed when calling StartPlayingAudio: ", err)
	}

	s.Log("Launching Settings app with Assistant query and waiting for it to open")
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatalf("Settings app did not appear in the shelf: %v. ", err)
	}
}

// copyFileToHost copies file from DUT to host.
// If dstFileName is not specified, then a temp location will be used.
// returns the actual location of where the file is copied
func copyFileToHost(ctx context.Context, hostname, srcFileName, dstFileName string) (string, error) {
	if dstFileName == "" {
		fileExt := filepath.Ext(srcFileName)
		fileName := strings.TrimSuffix(filepath.Base(srcFileName), fileExt)
		dstFileName = fmt.Sprintf("/tmp/%s%d%s", fileName, time.Now().Unix(), fileExt)
	}

	args := []string{"-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", srcFileName, fmt.Sprintf("root@%s:%s", hostname, dstFileName)}
	if out, err := testexec.CommandContext(ctx, "scp", args...).Output(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to copy file through scp. args:%v  stdout: %s", args, out)
	}
	return dstFileName, nil
}

// deleteFileFromHost deletes files from host
func deleteFileFromHost(ctx context.Context, hostname, sshPort, fileName string) error {
	hostAddr := fmt.Sprintf("%s:%s", hostname, sshPort)
	sopt := ssh.Options{
		ConnectTimeout: 10 * time.Second,
		WarnFunc:       func(msg string) { testing.ContextLog(ctx, msg) },
		Hostname:       hostAddr,
		User:           "root",
	}
	conn, err := ssh.New(ctx, &sopt)
	if err != nil {
		return errors.Wrapf(err, "failed to create SSH connection to host: %s", hostAddr)
	}
	defer conn.Close(ctx)
	args := []string{fileName}

	if out, err := conn.CommandContext(ctx, "rm", args...).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete file %s from host %s: stdout: %s", fileName, hostAddr, out)
	}
	return nil
}
