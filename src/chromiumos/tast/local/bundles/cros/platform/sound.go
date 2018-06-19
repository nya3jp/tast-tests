// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/release"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sound,
		Desc: "Checks that sound devices are recognized",
		Attr: []string{"informational"},
	})
}

func Sound(s *testing.State) {
	var (
		noPlayBoards = map[string]bool{
			"veyron_rialto": true,
		}
		noRecordBoards = map[string]bool{
			"veyron_mickey": true,
			"veyron_rialto": true,
		}
	)

	rel, err := release.Load()
	if err != nil {
		s.Fatal("Failed to load /etc/lsb-release: ", err)
	}

	canPlay := !noPlayBoards[rel.Board]
	canRecord := !noRecordBoards[rel.Board]

	// Check existence of device files.
	files, err := ioutil.ReadDir("/dev/snd")
	if err != nil {
		s.Fatal("Failed to list files at /dev/snd: ", err)
	}

	checkDevices := func(pattern string, mode os.FileMode) error {
		found := false
		p := regexp.MustCompile(pattern)
		for _, file := range files {
			if p.MatchString(file.Name()) {
				if file.Mode()&0777 != mode {
					return fmt.Errorf("permission mismatch: expected %o, actually %o: %s", mode, file.Mode(), file.Name())
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("no file matched %s", pattern)
		}
		return nil
	}

	if canPlay || canRecord {
		if err := checkDevices(`^controlC\d+$`, 0660); err != nil {
			s.Error(err)
		}
	}
	if canPlay {
		if err := checkDevices(`^pcmC\d+D\d+p$`, 0660); err != nil {
			s.Error(err)
		}
	}
	if canRecord {
		if err := checkDevices(`^pcmC\d+D\d+c$`, 0660); err != nil {
			s.Error(err)
		}
	}

	// Check alsa commands work.
	checkCommand := func(name string) error {
		cmd := testexec.CommandContext(s.Context(), name, "-l")
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd.DumpLog(s.Context())
			return fmt.Errorf("%s failed: %v", name, err)
		}
		if strings.Contains(string(out), "no soundcards found") {
			return fmt.Errorf("%s recognized no sound cards", name)
		}
		return nil
	}

	if canPlay {
		if err := checkCommand("aplay"); err != nil {
			s.Error(err)
		}
	}
	if canRecord {
		if err := checkCommand("arecord"); err != nil {
			s.Error(err)
		}
	}
}
