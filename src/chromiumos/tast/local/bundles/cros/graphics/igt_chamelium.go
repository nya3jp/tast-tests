// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

var (
	chameleonHostVar = testing.RegisterVarString(
		"graphics.chameleon_host",
		"",
		"Hostname for Chameleon (optional/not currently used)")

	chameleonIPVar = testing.RegisterVarString(
		"graphics.chameleon_ip",
		"",
		"IP address of Chameleon (required)")

	chameleonSSHPortVar = testing.RegisterVarString(
		"graphics.chameleon_ssh_port",
		"22",
		"SSH port for Chameleon (optional/not currently used)")

	chameleonPortVar = testing.RegisterVarString(
		"graphics.chameleon_port",
		"9992",
		"Port for chameleond on Chameleon (optional/used)")
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IgtChamelium,
		Desc: "Verifies IGT Chamelium test binaries run successfully",
		Contacts: []string{
			"chromeos-gfx-display@google.com",
			"markyacoub@google.com",
		},
		SoftwareDeps: []string{"drm_atomic", "igt", "no_qemu"},
		VarDeps:      []string{"graphics.chameleon_ip"},
		Attr:         []string{"group:graphics", "graphics_chameleon_igt"},
		Fixture:      "chromeGraphicsIgt",
		Params: []testing.Param{{
			Name: "kms_chamelium",
			Val: graphics.IgtTest{
				Exe: "kms_chamelium",
			},
			Timeout:   15 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}},
	})
}

func setIgtrcFile(s *testing.State) {
	igtFilePath := "/tmp/.igtrc"
	igtFile, err := os.Create(igtFilePath)
	if err != nil {
		s.Fatal("Failed to create .igtrc: ", err)
		return
	}
	defer igtFile.Close()

	// Get Chameleon IP
	// This is used for local dev env.
	s.Log("Got testing.RegisterVarString")
	chameleonIP := ""
	if chameleonIPVar != nil {
		addr := net.ParseIP(chameleonIPVar.Value())
		if addr == nil {
			s.Fatal("Failed to get chameleon ip. The Chameleon ip: ", addr)
		}
		chameleonIP = chameleonIPVar.Value()
	}

	chameleonPort := ""
	if chameleonPortVar != nil {
		chameleonPort = chameleonPortVar.Value()
	}

	url := chameleonIP + ":" + chameleonPort
	content := `
[Common]
# The path to dump frames that fail comparison checks
FrameDumpPath=/tmp

[DUT]
SuspendResumeDelay=15

[Chamelium]
URL=` + url + `

`

	// Write config content to .igtrc
	_, err = igtFile.WriteString(content)
	if err != nil {
		s.Fatal("Failed to write to igtrc: ", err)
		return
	}

	// Set the file path as env variable for IGT to find it.
	os.Setenv("IGT_CONFIG_PATH", igtFilePath)

	s.Log("The igtFilePath is ", igtFilePath)
	s.Log("The url is ", url)
}

func IgtChamelium(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(graphics.IgtTest)
	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.Exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	setIgtrcFile(s)

	testPath := filepath.Join("chamelium", testOpt.Exe)
	isExitErr, exitErr, err := graphics.IgtExecuteTests(ctx, testPath, f)

	isError, outputLog := graphics.IgtProcessResults(testOpt.Exe, f, isExitErr, exitErr, err)

	if isError {
		s.Error(outputLog)
	} else {
		s.Log(outputLog)
	}
}
