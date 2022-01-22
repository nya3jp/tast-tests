// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	remoteTi50 "chromiumos/tast/remote/firmware/ti50"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50SystemTestImage,
		Desc:    "Ti50 system test",
		Timeout: 5 * time.Minute,
		Vars:    []string{"image", "spiflash", "mode"},
		Contacts: []string{
			"ecgh@chromium.org",
			"ti50-core@google.com",
		},
		ServiceDeps: []string{"tast.cros.baserpc.FileSystem", "tast.cros.firmware.SerialPortService"},
		Attr:        []string{"group:firmware"},
	})
}

func Ti50SystemTestImage(ctx context.Context, s *testing.State) {

	mode, _ := s.Var("mode")
	spiflash, _ := s.Var("spiflash")

	board, rpcClient, err := remoteTi50.GetTi50TestBoard(ctx, s.DUT(), s.RPCHint(), mode, spiflash, 10000, time.Second)

	if err != nil {
		s.Fatal("GetTi50TestBoard: ", err)
	}
	if rpcClient != nil {
		defer rpcClient.Close(ctx)
	}
	defer board.Close(ctx)

	image, _ := s.Var("image")
	if image == "" {
		if err = board.Reset(ctx); err != nil {
			s.Fatal("Failed to reset: ", err)
		}
	} else {
		if err = board.FlashImage(ctx, image); err != nil {
			s.Fatal("Failed spiflash: ", image, err)
		}
	}

	s.Log("Kernel tests:")
	failCount := checkTestResults(ctx, s, board, "KERNEL")

	s.Log("App tests:")
	failCount += checkTestResults(ctx, s, board, "APP")

	if failCount > 0 {
		s.Fatalf("%d test failures", failCount)
	}
}

func checkTestResults(ctx context.Context, s *testing.State, board ti50.DevBoard, sectionName string) int {
	failCount := 0
	_, err := board.ReadSerialSubmatch(ctx, regexp.MustCompile("##"+sectionName+" TESTS START"))
	if err != nil {
		s.Fatal("Failed to read section start: ", err)
	}
	endMarker := "##" + sectionName + " TESTS END"
	re := regexp.MustCompile("(" + endMarker + `|##TEST (SKIP|START) (\S+)\s)`)
	for {
		m, err := board.ReadSerialSubmatch(ctx, re)
		if err != nil {
			s.Fatal("Failed to read next test: ", err)
		}
		match := string(m[0])
		if match == endMarker {
			return failCount
		}
		start := string(m[2])
		testName := string(m[3])
		result := "Skip"
		if start != "SKIP" {
			result, err = waitForTest(ctx, s, board, testName)
			if err != nil {
				s.Fatal("Failed to read test result: ", err)
			}
		}
		if result == "Fail" {
			failCount++
		}
	}
}

func waitForTest(ctx context.Context, s *testing.State, board ti50.DevBoard, testName string) (result string, err error) {
	lineRe := regexp.MustCompile(`.*[\r\n]+`)
	slowCryptoRe := regexp.MustCompile("Long running SW crypto operation")
	resultRe := regexp.MustCompile("##TEST RESULT " + testName + `: (\S+)`)
	timeLimit := 50 * time.Second
	testTime := time.Now()
	line := ""
	lineTime := time.Now()
	for {
		m, err := board.ReadSerialSubmatch(ctx, lineRe)
		if err == nil {
			delay := time.Now().Sub(lineTime)
			if delay > 10*time.Second {
				s.Logf("(%q took %ds)", line, int(delay.Seconds()))
			}
			lineTime = time.Now()
			line = strings.TrimSpace(string(m[0]))
			m := resultRe.FindStringSubmatch(line)
			if m != nil {
				elapsed := time.Now().Sub(testTime)
				result := m[1]
				s.Logf("%s: %s (%ds)", testName, result, int(elapsed.Seconds()))
				return result, nil
			}
			m = slowCryptoRe.FindStringSubmatch(line)
			if m != nil {
				timeLimit += 5 * time.Minute
				s.Log("(Waiting for slow crypto.)")
			}
		} else {
			elapsed := time.Now().Sub(testTime)
			if elapsed > timeLimit {
				s.Logf("Still waiting for test %s after %ds, giving up", testName, int(elapsed.Seconds()))
				delay := time.Now().Sub(lineTime)
				s.Logf("Waited %ds at %q", int(delay.Seconds()), line)
				return "", err
			}
		}
	}
}
