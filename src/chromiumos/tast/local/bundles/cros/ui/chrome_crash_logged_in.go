// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// chromeCrashLoggedInParams contains the test parameters which are different between the various tests.
type chromeCrashLoggedInParams struct {
	ptype         chromecrash.ProcessType
	handler       chromecrash.CrashHandler
	consent       crash.ConsentType
	restartChrome bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while logged in",
		Contacts:     []string{"iby@chromium.org", "cros-telemetry@google.com"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "browser_breakpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
		}, {
			Name: "browser_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "browser_crashpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"crashpad", "metrics_consent"},
		}, {
			Name: "browser_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"crashpad"},
		}, {
			Name: "gpu_process_breakpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
				// Breakpad only adds a signal handler to a process if it has consent
				// at the time the process starts up. There's a special hook in the
				// browser process to initialize Breakpad if consent is given after
				// startup
				// (https://chromium-review.googlesource.com/c/chromium/src/+/2145676);
				// however, this only works for the browser process. For the GPU
				// process, we need to set consent and then restart Chrome. The
				// mock_consent variants use a command-line argument which forces the
				// 'install a signal handler' behavior, so we only need this for the
				// real consent variant.
				restartChrome: true,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
		}, {
			Name: "gpu_process_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "gpu_process_crashpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"crashpad", "metrics_consent"},
		}, {
			Name: "gpu_process_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"crashpad"},
		}, {
			Name: "broker_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Broker,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraAttr: []string{"group:mainline"},
			// If the gpu process is not sandboxed, it will not create a broker.
			ExtraSoftwareDeps: []string{"breakpad", "gpu_sandboxing"},
		}, {
			Name: "broker_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Broker,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
			// If the gpu process is not sandboxed, it will not create a broker.
			ExtraSoftwareDeps: []string{"crashpad", "gpu_sandboxing"},
		}},
	})
}

func ChromeCrashLoggedIn(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashLoggedInParams)
	ct, err := chromecrash.NewCrashTester(ctx, params.ptype, chromecrash.MetaFile)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	extraArgs := chromecrash.GetExtraArgs(params.handler, params.consent)
	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer func() {
		if cr != nil {
			cr.Close(ctx)
		}
	}()

	opt := crash.WithMockConsent()
	if params.consent == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if params.restartChrome {
		cr.Close(ctx)
		// Need to KeepState to avoid erasing the consent we just set up.
		cr, err = chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(extraArgs...))
		if err != nil {
			cr = nil
			s.Fatal("Chrome login failed: ", err)
		}
	}

	files, err := ct.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", params.ptype, err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Errorf("Crash files weren't written to cryptohome after crashing the %s process: %v", params.ptype, err)
		// So we've seen weird problems where the meta files get created but by the
		// time 'newFiles, err := crash.GetCrashes(dirs...)' runs inside
		// KillAndGetCrashFiles, the meta files aren't found. Add more debugging
		// output to diagnose. crbug.com/1080365
		args := []string{"-lia", "/home/chronos", "/home/user"}
		if paths, err := filepath.Glob(chromecrash.CryptohomePattern); err != nil {
			s.Log("Error getting cryptohomes from ", chromecrash.CryptohomePattern, ": ", err)
		} else {
			for _, path := range paths {
				args = append(args, path, filepath.Join(path, "crash"))
			}
		}
		if paths, err := filepath.Glob("/home/user/*"); err != nil {
			s.Log("Error getting cryptohomes from /home/user/*: ", err)
		} else {
			for _, path := range paths {
				args = append(args, path, filepath.Join(path, "crash"))
			}
		}

		cmd := testexec.CommandContext(ctx, "/bin/ls", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			s.Logf("ls of %v failed: %v", args, err)
		}
		outfile := filepath.Join(s.OutDir(), "ls_output.txt")
		if err := ioutil.WriteFile(outfile, out, 0644); err != nil {
			s.Logf("Storing ls output %v to %v failed: %v", out, outfile, err)
		}
	}
}
