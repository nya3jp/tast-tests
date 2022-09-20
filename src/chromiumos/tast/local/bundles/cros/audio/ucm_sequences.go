// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UCMSequences,
		Desc:     "Exercise UCM config enable/disable sequences",
		Contacts: []string{"aaronyu@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline"},
		Timeout:  1 * time.Minute,
		HardwareDeps: hwdep.D(
			// TODO(b/231276793): eve hotword broken.
			hwdep.SkipOnModel("eve"),
		),
		Fixture: "crasStopped",
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
	param := s.Param().(ucmSequencesParam)

	cards, err := audio.GetSoundCards()
	if err != nil {
		s.Fatal("Failed to get sound cards: ", err)
	}

	boardConfig, err := audio.LoadBoardConfig(ctx)
	if err != nil {
		s.Fatal("Failed to load board config: ", err)
	}

	ucmSuffix, err := crosconfig.Get(ctx, "/audio/main", "ucm-suffix")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Cannot get ucm suffix: ", err)
	}

	for _, card := range cards {
		isExternal, err := card.IsExternal()
		if err != nil {
			s.Errorf("Cannot tell if %s is an external card: %s", card.ShortName, err)
			continue
		}
		if isExternal {
			s.Logf("Skipping external card %s", card.ShortName)
			continue
		}

		ucmName := card.ShortName
		if ucmSuffix != "" && !boardConfig.ShouldIgnoreUCMSuffix(card.ShortName) {
			ucmName += "." + ucmSuffix
		}
		ucmSequencesTestCard(ctx, s, param.alsaucmCommander, ucmName)
	}
}

func ucmSequencesTestCard(ctx context.Context, s *testing.State, c alsaucmCommander, ucmName string) {
	const ucmBasePath = "/usr/share/alsa/ucm"

	ucmConf := filepath.Join(ucmBasePath, ucmName, ucmName+".conf")
	s.Logf("Testing %s UCM: %s", ucmName, ucmConf)

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
