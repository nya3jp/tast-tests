// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioDecoder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that Assistant Audio Decoder service starts on demand",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantWithStartAudioDecoderOnDemand",
		Timeout:      5 * time.Minute,
	})
}

const audioDecoderUtilProcName = "ash.assistant.mojom.AssistantAudioDecoderFactory"

type audioDecoderUtilProcNotFoundError struct {
	utilProcNames []string
}

func (e *audioDecoderUtilProcNotFoundError) Error() string {
	return fmt.Sprintf("%s not found in: %v", audioDecoderUtilProcName, e.utilProcNames)
}

type audioDecoderUtilProcFoundError struct{}

func (e *audioDecoderUtilProcFoundError) Error() string {
	return fmt.Sprintf("%s found", audioDecoderUtilProcName)
}

func findAudioDecoderUtilityProcess() (*process.Process, error) {
	procs, err := chromeproc.GetUtilityProcesses()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get utility processes")
	}

	re := regexp.MustCompile(` --?utility-sub-type=([\w\.]+)(?: |$)`)

	// Store utility process names for generating an error message.
	var utilProcNames []string
	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cmdline")
		}

		matches := re.FindStringSubmatch(cmdline)
		if len(matches) < 2 {
			continue
		}

		procName := matches[1]
		utilProcNames = append(utilProcNames, procName)

		if procName == audioDecoderUtilProcName {
			return proc, nil
		}
	}
	return nil,
		&audioDecoderUtilProcNotFoundError{utilProcNames: utilProcNames}
}

func expectNoAudioDecoderUtilityProcess() error {
	_, err := findAudioDecoderUtilityProcess()
	if _, ok := err.(*audioDecoderUtilProcNotFoundError); ok {
		return nil
	} else if err == nil {
		return &audioDecoderUtilProcFoundError{}
	}
	return err
}

func expectAudioDecoderUtilityProcess() error {
	_, err := findAudioDecoderUtilityProcess()
	return err
}

func AudioDecoder(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome
	accel, err := assistant.ResolveAssistantHotkey(s.Features(""))
	if err != nil {
		s.Fatal("Failed to resolve assistant hotkey: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectNoAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect no audio decoder process is running: ", err)
	}

	// assistant.SendTextQuery waits a response (e.g. text response) from Assistant.
	// Play News query does not provide those type of responses and the API call gets timed out.
	// Use SendTextQueryViaUI as it does not wait a response.
	if err := assistant.SendTextQueryViaUI(ctx, tconn, "Play News", accel); err != nil {
		s.Fatal("Failed to send text query via UI: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect audio decoder process is running: ", err)
	}

	_, err = assistant.SendTextQuery(ctx, tconn, "Stop News")
	if err != nil {
		s.Fatal("Failed to send Stop News query: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectNoAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect no audio decoder process is running: ", err)
	}
}
