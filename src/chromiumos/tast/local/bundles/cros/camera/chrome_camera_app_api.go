// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCameraAppAPI,
		Desc:         "Verifies that the private JavaScript APIs CCA relies on work as expected",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.USBCamera, "chrome_login"},
		Data:         append([]string{}),
	})
}

// ChromeCameraAppAPI verifies whether the private JavaScript APIs CCA relies on work as expected.
// The APIs under testing are not owned by CCA team. The test is used to prevent the implementation
// changes of those APIs silently break CCA.
func ChromeCameraAppAPI(ctx context.Context, s *testing.State) {
	chromeArgs := []string{
		"--use-fake-ui-for-media-stream",
		"--use-fake-device-for-media-stream",
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	bgURL := chrome.ExtensionBackgroundPageURL(ccaID)
	s.Log("Connecting to CCA background ", bgURL)
	ccaConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}

	if err := ccaConn.WaitForExpr(ctx, "chrome.fileManagerPrivate"); err != nil {
		s.Fatal("Failed to wait for expression: ", err)
	}
	s.Log("Connected to CCA background")

	runAllTests(ctx, s, ccaConn)
}

func runAllTests(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	testCanAccessExternalStorage(ctx, s, conn)
	// TODO(shenghao): Add tests for other private APIs.
}

func testCanAccessExternalStorage(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	entryExist := false
	if err := conn.EvalPromise(ctx,
		`
      new Promise((resolve, reject) => {
        chrome.fileSystem.getVolumeList((volumes) => {
          if (volumes) {
            for (var i = 0; i < volumes.length; i++) {
              var volumeId = volumes[i].volumeId;
              if (volumeId.indexOf('downloads:Downloads') !== -1 ||
                  volumeId.indexOf('downloads:MyFiles') !== -1) {
                chrome.fileSystem.requestFileSystem(volumes[i],
                    (fs) => resolve([fs && fs.root, volumeId]));
                return;
              }
            }
          }
          resolve([null, null]);
        });
      }).then(([dir, volumeId]) => {
        if (volumeId && volumeId.indexOf('downloads:MyFiles') !== -1) {
          readDir = (dir) => {
            return !dir ? Promise.resolve([]) : new Promise((resolve, reject) => {
              var dirReader = dir.createReader();
              var entries = [];
              var readEntries = () => {
                dirReader.readEntries((inEntries) => {
                  if (inEntries.length == 0) {
                    resolve(entries);
                    return;
                  }
                  entries = entries.concat(inEntries);
                  readEntries();
                }, reject);
              };
              readEntries();
            });
          }
          return readDir(dir).then((entries) => {
            return entries.find(
                (entry) => entry.name == 'Downloads' && entry.isDirectory) != null;
          });
          return true;
        }
        return dir != null;
      })
    `, &entryExist); err != nil {
		s.Fatal("Failed to evaluate promise: ", err)
	}
	if !entryExist {
		s.Fatal("Failed to access the designated external storage")
	}
}
