// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type toolkitConfig struct {
	data          string
	deps          []string
	command       []string
	appID         string
	expectedColor color.Color
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Toolkit,
		Desc:     "Verifies the behaviour of GUI apps based on various toolkits",
		Contacts: []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"informational"},
		Params: []testing.Param{{
			Name:      "gtk3_wayland",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: toolkitConfig{
				data:          "toolkit_gtk3_demo.py",
				deps:          []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command:       []string{"GDK_BACKEND=wayland", "python3", "toolkit_gtk3_demo.py"},
				appID:         "crostini:toolkit_gtk3_demo.py",
				expectedColor: colorcmp.RGB(255, 255, 0),
			}}, {
			Name:      "gtk3_x11",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: toolkitConfig{
				data:          "toolkit_gtk3_demo.py",
				deps:          []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command:       []string{"GDK_BACKEND=x11", "python3", "toolkit_gtk3_demo.py"},
				appID:         "crostini:org.chromium.termina.wmclass.Toolkit_gtk3_demo.py",
				expectedColor: colorcmp.RGB(255, 255, 0),
			}}, {
			Name:      "qt5",
			ExtraData: []string{"toolkit_qt5_demo.py"},
			Val: toolkitConfig{
				data:          "toolkit_qt5_demo.py",
				deps:          []string{"python3-pyqt5"},
				command:       []string{"python3", "toolkit_qt5_demo.py"},
				appID:         "crostini:org.chromium.termina.wmclass.toolkit_qt5_demo.py",
				expectedColor: colorcmp.RGB(255, 0, 255),
			}}, {
			Name:      "tkinter",
			ExtraData: []string{"toolkit_tkinter_demo.py"},
			Val: toolkitConfig{
				data:          "toolkit_tkinter_demo.py",
				deps:          []string{"python3-tk"},
				command:       []string{"python3", "toolkit_tkinter_demo.py"},
				appID:         "crostini:org.chromium.termina.wmclass.Tkinter_demo",
				expectedColor: colorcmp.RGB(0, 255, 255),
			},
		}},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func Toolkit(ctx context.Context, s *testing.State) {
	conf := s.Param().(toolkitConfig)
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := pre.Container

	dataPath := "/home/testuser/" + conf.data
	if err := cont.PushFile(ctx, s.DataPath(conf.data), dataPath); err != nil {
		s.Fatalf("Failed to push %v to container: %v", conf.data, err)
	}

	if len(conf.deps) > 0 {
		s.Log("Installing: ", strings.Join(conf.deps, " "))
		installArgs := []string{"sudo", "apt-get", "-y", "install"}
		installArgs = append(installArgs, conf.deps...)
		if err := cont.Command(ctx, installArgs...).Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to install %s: %v", strings.Join(conf.deps, " "), err)
		}
	}

	s.Log("Running the demo")
	cmd := cont.Command(ctx, "sh", "-c", shutil.EscapeSlice(conf.command))
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to start %q: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	// We defer Wait() without Kill() as doing otherwise allows the kill
	// signal to hide errors (such as that we couldnt close the app).
	// Instead we time-out on the Wait(), so that an error is generated.
	defer cmd.Wait(testexec.DumpLogOnError)
	defer func() {
		if err := closeApplication(ctx, s, tconn, conf.appID); err != nil {
			s.Fatalf("Failed to close application %q: %v", conf.appID, err)
		}
	}()

	if err := crostini.MatchScreenshotDominantColor(ctx, cr, conf.expectedColor, filepath.Join(s.OutDir(), "screenshot.png")); err != nil {
		s.Fatal("Failed during screenshot check: ", err)
	}
}

func closeApplication(ctx context.Context, s *testing.State, tconn *chrome.Conn, appID string) error {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.closeApp('%v', () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appID)
	return tconn.EvalPromise(ctx, expr, nil)
}
