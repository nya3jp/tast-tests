// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const runAplayRerunRestartCras string = "run-aplay-rerun-restart-cras.sh"

type audioAplayRerunRestartCrasParams struct {
	crosvmArgs    []string
	vhostUserArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioAplayRerunRestartCras,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that if the cras is restarted while aplay is playing, aplay can still be run again later",
		Contacts:     []string{"pteerapong@chromium.org", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAplayRerunRestartCras},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{
			{
				Name: "virtio_cras_snd",
				Val: audioAplayRerunRestartCrasParams{
					crosvmArgs: []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy"},
				},
			},
		},
	})
}

func AudioAplayRerunRestartCras(ctx context.Context, s *testing.State) {
	param := s.Param().(audioAplayRerunRestartCrasParams)
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	outputLogPath := filepath.Join(s.OutDir(), "output.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runAplayRerunRestartCras)),
		"--",
		outputLogPath,
	}

	config := audioutils.Config{
		CrosvmArgs:    param.crosvmArgs,
		VhostUserArgs: param.vhostUserArgs,
	}

	var runScriptWG sync.WaitGroup
	runScriptWG.Add(1)
	go func() {
		defer runScriptWG.Done()
		if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
			s.Fatal("Failed to run crosvm: ", err)
		}
	}()

	testing.Sleep(ctx, 5*time.Second)
	testing.ContextLog(ctx, "Restarting cras")
	if err := testexec.CommandContext(ctx, "restart", "cras").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to restart cras: ", err)
	}

	// Wait for script to finish and read result
	testing.ContextLog(ctx, "Waiting for the script to finish")
	runScriptWG.Wait()

	// Read output and check that the second aplay returned 0
	output, err := ioutil.ReadFile(outputLogPath)
	if err != nil {
		s.Fatal("Failed to read output file: ", err)
	}
	if !strings.Contains(string(output), "aplay 2 returned 0") {
		s.Fatal("aplay error after restart cras")
	}
}
