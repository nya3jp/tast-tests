// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type config struct {
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
			Val: config{
				data:          "toolkit_gtk3_demo.py",
				deps:          []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command:       []string{"GDK_BACKEND=wayland", "python3", "toolkit_gtk3_demo.py"},
				appID:         "crostini:toolkit_gtk3_demo.py",
				expectedColor: colorcmp.RGB(255, 255, 0),
			}}, {
			Name:      "gtk3_x11",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: config{
				data:          "toolkit_gtk3_demo.py",
				deps:          []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command:       []string{"GDK_BACKEND=x11", "python3", "toolkit_gtk3_demo.py"},
				appID:         "crostini:org.chromium.termina.wmclass.Toolkit_gtk3_demo.py",
				expectedColor: colorcmp.RGB(255, 255, 0),
			}}, {
			Name:      "qt5",
			ExtraData: []string{"toolkit_qt5_demo.py"},
			Val: config{
				data:          "toolkit_qt5_demo.py",
				deps:          []string{"python3-pyqt5"},
				command:       []string{"python3", "toolkit_qt5_demo.py"},
				appID:         "crostini:org.chromium.termina.wmclass.toolkit_qt5_demo.py",
				expectedColor: colorcmp.RGB(255, 0, 255),
			}}, {
			Name:      "tkinter",
			ExtraData: []string{"toolkit_tkinter_demo.py"},
			Val: config{
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
	conf := s.Param().(config)
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
		installSlice := []string{"sudo", "apt-get", "-y", "install"}
		installSlice = append(installSlice, conf.deps...)
		if err := cont.Command(ctx, installSlice...).Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to install %s: %v", strings.Join(conf.deps, " "), err)
		}
	}

	s.Log("Running the demo")
	cmd := cont.Command(ctx, "sh", "-c", shutil.EscapeSlice(conf.command))
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to start %q: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	defer cmd.Wait(testexec.DumpLogOnError)

	if err := crostini.PollScreenshotDominantColor(ctx, s, cr, conf.expectedColor); err != nil {
		s.Fatal("Failed to see screenshot: ", err)
	}

	if err := closeApplication(ctx, s, tconn, conf.appID); err != nil {
		s.Fatalf("Failed to close application %q: %v", conf.appID, err)
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
