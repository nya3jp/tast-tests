// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIStress,
		Desc:         "",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      3000 * time.Minute,
	})
}

func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}

func CCAUIStress(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// MODIFY HERE TO SWITCH BETWEEN CCA/WebRtc DEMO.
	testCCA := false
	if testCCA {
		for {
			app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
			if err != nil {
				s.Fatal("Failed to open CCA: ", err)
			}

			if err := app.WaitForVideoActive(ctx); err != nil {
				s.Fatal("Preview is inactive after launching App: ", err)
			}

			if err := app.SwitchMode(ctx, cca.Video); err != nil {
				s.Fatal("Failed to switch to video mode: ", err)
			}

			fileInfo, err := app.RecordVideo(ctx, cca.TimerOff, 10 * time.Second)
			if err != nil {
				s.Fatal("Failed to record video: ", err)
			}

			dir, err := cca.GetSavedDir(ctx, cr)
			if err != nil {
				s.Fatal("Failed to get saved dir: ", err)
			}

			err = os.Remove(dir + "/" + fileInfo.Name())
			if err != nil {
				s.Fatal("Failed to delete file: ", err)
			}

			if err := app.Close(ctx); err != nil {
				s.Fatal("Failed to close app: ", err)
			}

			testing.Sleep(ctx, 2 * time.Second)
		}
	} else {
		dummyConn, _ := cr.NewConn(ctx, "https://www.google.com")
		defer dummyConn.Close()
		defer dummyConn.CloseTarget(ctx)

		for {
			conn, err := cr.NewConn(ctx, "https://webrtc.github.io/samples/src/content/getusermedia/record/")
			if err != nil {
				s.Fatal("Failed to connect to webpage: ", err)
			}

			if err := conn.EvalPromise(ctx, `
				new Promise(outerResolve => {
					async function run() {
						const startBtn = document.getElementById('start');
						if (startBtn === null) {
							throw new Error("Failed to click start button");
						}
						startBtn.click();

						await new Promise(resolve => {
							var checkEnableInterval = setInterval(() => {
								if (!document.getElementById('record').disabled) {
									resolve();
								}
							}, 100);
						});

						const recordBtn = document.getElementById('record');
						if (recordBtn === null) {
							throw new Error("Failed to click record button");
						}
						recordBtn.click();

						await new Promise(resolve => setTimeout(resolve, 10000));

						recordBtn.click();

						const downloadBtn = document.getElementById('download');
						if (downloadBtn === null) {
							throw new Error("Failed to click download button");
						}
						downloadBtn.click();

						outerResolve();
					}

					if (document.readyState === "complete") {
						run();
					} else {
						document.addEventListener('DOMContentLoaded', run);
					}
				});
			`, nil); err != nil {
				s.Fatal("Failed to run js script: ", err)
			}

			if err := conn.CloseTarget(ctx); err != nil {
				s.Fatal("Failed to close target")
			}
			if err := conn.Close(); err != nil {
				s.Fatal("Failed to close connection")
			}
			testing.ContextLogf(ctx, "Sleeping for a while")
			testing.Sleep(ctx, 2 * time.Second)
		}
	}
}
