// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UCMSequences,
		Desc:     "Exercise UCM config enable/disable sequences",
		Contacts: []string{"aaronyu@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  1 * time.Minute,
		HardwareDeps: hwdep.D(
			// TODO(b/231276793): eve hotword broken.
			hwdep.SkipOnModel("eve"),
		),
		Params: []testing.Param{
			{
				Name: "section_verb",
				Val: ucmSequencesParam{
					alsaucmCommander: staticCommander{
						{
							name:      "SectionVerb",
							extraArgs: nil,
						},
					},
				},
			},
			{
				Name: "section_device",
				Val: ucmSequencesParam{
					alsaucmCommander: listCommander{
						section: "_devices/HiFi",
						enable:  "_enadev",
						disable: "_disdev",
					},
				},
			},
			{
				Name: "section_modifier",
				Val: ucmSequencesParam{
					alsaucmCommander: listCommander{
						section: "_modifiers/HiFi",
						enable:  "_enamod",
						disable: "_dismod",
					},
				},
			},
		},
	})
}

type ucmSequencesParam struct {
	alsaucmCommander
}

// alsaucmCommander provides commands() that returns a list of alsaucmCommand,
// which can be used to generate alsaucm commands to run.
type alsaucmCommander interface {
	commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error)
}

type alsaucmCommand struct {
	name      string   // human readable name describing the command
	extraArgs []string // extra args after `alsaucm -c$cardname set _verb HiFi`
}

// staticCommander is an alsaucmCommander that returns a predetermined list of alsaucmCommand.
type staticCommander []alsaucmCommand

var _ alsaucmCommander = staticCommander{}

func (c staticCommander) commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error) {
	return c, nil
}

// listCommander is an alsaucmCommander that returns a list of alsaucmCommand
// based on available sections in HiFi.conf.
type listCommander struct {
	section string
	enable  string
	disable string
}

var _ alsaucmCommander = listCommander{}

func (p listCommander) commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error) {
	cmd := testexec.CommandContext(ctx,
		"alsaucm",
		"-c"+ucmName,
		"list",
		p.section,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list modifiers")
	}

	// Find the names listed after the ": "
	// example:
	// 0: Hotword Model ar_sa
	// 1: Hotword Model cmn
	var commands []alsaucmCommand
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			name := parts[1]
			commands = append(commands, alsaucmCommand{name, []string{"set", p.enable, name}})
			commands = append(commands, alsaucmCommand{name, []string{"set", p.disable, name}})
		}
	}
	return commands, nil
}

func UCMSequences(ctx context.Context, s *testing.State) {
	// Restart CRAS after running the test.
	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(clearUpCtx, 5*time.Second)
	defer cancel()
	defer audio.RestartCras(clearUpCtx)

	// Stop cras and sleep to get exclusive access to audio devices.
	if err := upstart.StopJob(ctx, "cras"); err != nil {
		s.Fatal("Cannot stop cras: ", err)
	}
	// Sleep required for pm_runtime_set_autosuspend_delay(&pdev->dev, 10000)
	// https://source.chromium.org/chromium/chromiumos/third_party/kernel/+/HEAD:sound/soc/amd/acp-pcm-dma.c;l=1271;drc=662fb3efe7ee835f0eeba6bc63b81e82a97fc312
	s.Log("Sleeping 11 seconds to wait for audio device to be ready")
	testing.Sleep(ctx, 11*time.Second)

	param := s.Param().(ucmSequencesParam)

	cards, err := audio.GetSoundCards()
	if err != nil {
		s.Fatal("Failed to get sound cards: ", err)
	}

	ucmSuffix, err := crosconfig.Get(ctx, "/audio/main", "ucm-suffix")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Cannot get ucm suffix: ", err)
	}

	for _, card := range cards {
		ucmName := card.ShortName
		if ucmSuffix != "" {
			ucmName += "." + ucmSuffix
		}
		ucmSequencesTestCard(ctx, s, param.alsaucmCommander, ucmName)
	}
}

func ucmSequencesTestCard(ctx context.Context, s *testing.State, c alsaucmCommander, ucmName string) {
	const ucmBasePath = "/usr/share/alsa/ucm"

	s.Logf("Testing UCM: %s", ucmName)

	// Check we have a complete HiFi.conf.
	// If the file is absent or empty (placeholder during bringup), skip the test.
	hifiConf := filepath.Join(ucmBasePath, ucmName, "HiFi.conf")
	if stat, err := os.Stat(hifiConf); err != nil {
		if os.IsNotExist(err) {
			s.Log("Skipping due to missing HiFi.conf")
			return
		}
		if stat.Size() == 0 {
			s.Log("Skipping due to empty HiFi.conf")
			return
		}
		s.Fatal("Failed to stat HiFi.conf: ", err)
	}

	ucmCmds, err := c.commands(ctx, ucmName)
	if err != nil {
		s.Error("Cannot get UCM commands: ", err)
		return
	}
	for _, ucmCmd := range ucmCmds {
		cmd := testexec.CommandContext(ctx,
			"alsaucm",
			"-c"+ucmName,
			"set",
			"_verb",
			"HiFi",
		)
		cmd.Args = append(cmd.Args, ucmCmd.extraArgs...)
		cmdStr := formatCommand(cmd)
		s.Log("Running ", cmdStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			s.Errorf("Command %s failed:%s%s", cmdStr, "\n", string(out))
		}
	}
}

// formatCommand formats a command for it to be printed to the user.
func formatCommand(cmd *testexec.Cmd) string {
	var b strings.Builder
	b.WriteString(shellQuote(cmd.Path))
	for _, arg := range cmd.Args[1:] {
		b.WriteByte(' ')
		b.WriteString(shellQuote(arg))
	}
	return b.String()
}

var simpleStringRegexp = regexp.MustCompile(`^[a-zA-Z0-9_\-/\.]+$`)

// shellQuote quotes a string so it is a single string token in shell.
// It does not try to use minimal quotes.
func shellQuote(s string) string {
	if simpleStringRegexp.MatchString(s) {
		return s
	}
	return fmt.Sprintf(`'%s'`, strings.ReplaceAll(s, `'`, `\'`))
}
