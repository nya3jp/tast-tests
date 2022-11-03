// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/capturemode"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type dragDropParams struct {
	count       int
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeBrowserDragAndDrop,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks dragging and dropping files out of Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "single_drag_and_drop",
			Val: dragDropParams{
				count:       1,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "multiple_drag_and_drop",
			Val: dragDropParams{
				count:       2,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "lacros_single_drag_and_drop",
			Val: dragDropParams{
				count:       1,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_multiple_drag_and_drop",
			Val: dragDropParams{
				count:       2,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

const dragAndDropHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Drag And Drop</title>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
      #drop-area {
        border: 2px dashed #ccc;
        border-radius: 20px;
        height: 75vh;
        width: 75vh;
        font-family: sans-serif;
        margin: 20px auto;
        padding: 20px;
      }
      #drop-area.highlight {
        border-color: purple;
      }
      p {
        margin-top: 0;
      }
      #fileList {
        width: 150px;
        margin-bottom: 10px;
        margin-right: 10px;
        vertical-align: middle;
      }
    </style>
    <script type="text/javascript">
      document.addEventListener('DOMContentLoaded', (event) => {

        function handleDragOver(e) {
          e.preventDefault();
          e.dataTransfer.dropEffect = 'copy';
          return false;
        }

        function handleDragEnter(e) {
          this.classList.add('over');
        }

        function handleDragLeave(e) {
          this.classList.remove('over');
        }

        function handleDrop(e) {
          e.stopPropagation();

          let dt = e.dataTransfer;
          let files = dt.files;

          handleFiles(files);
          return false;
        }

        function handleFiles(files) {
          ([...files]).forEach(displayFileName);
        }

        function displayFileName(file) {
          const node = document.createElement("li");
          const textnode = document.createTextNode(file.name);
          node.appendChild(textnode);
          document.getElementById('fileList').appendChild(node);
        }

        function highlight(e) {
          dropArea.classList.add('highlight')
        }

        function unhighlight(e) {
          dropArea.classList.remove('highlight')
        }

        function preventDefaults (e) {
          e.preventDefault()
          e.stopPropagation()
        }

        let dropArea = document.getElementById('drop-area');

        dropArea.addEventListener('dragenter', handleDragEnter, false);
        dropArea.addEventListener('dragleave', handleDragLeave, false);
        dropArea.addEventListener('dragover', handleDragOver, false);
        dropArea.addEventListener('drop', handleDrop, false);

        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
          dropArea.addEventListener(eventName, preventDefaults, false)
        });

        ['dragenter', 'dragover'].forEach(eventName => {
          dropArea.addEventListener(eventName, highlight, false)
        });

        ['dragleave', 'drop'].forEach(eventName => {
          dropArea.addEventListener(eventName, unhighlight, false)
        });
      });
    </script>
  </head>
  <body>
    <div id="drop-area">
      <p>Drag and drop files onto the dashed region to display dropped file(s)</p>
      <p id="fileList"></p>
  </div>
  </body>
</html>
`

// ChromeBrowserDragAndDrop tests the functionality of dragging and dropping single/multiple files from Holding Space to a Chrome Browser window.
func ChromeBrowserDragAndDrop(ctx context.Context, s *testing.State) {
	params := s.Param().(dragDropParams)
	bt := params.browserType

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Connect to a fresh ash-chrome instance (cr) to ensure holding space first-run state,
	// also get a browser instance (br) for browser functionality in common.
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig())
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)
	defer closeBrowser(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	screenshotLocations := make([]string, params.count)

	// Create number of screenshots needed.
	var screenshotNames []string
	for i := 0; i < params.count; i++ {
		screenshotLocations[i], err = capturemode.TakeScreenshot(ctx, downloadsPath)
		if err != nil {
			s.Fatal("Failed to capture first screenshot: ", err)
		}
		defer os.Remove(screenshotLocations[i])
		screenshotNames = append(screenshotNames, filepath.Base(screenshotLocations[i]))
	}

	uia := uiauto.New(tconn)

	uia.LeftClick(holdingspace.FindTray())(ctx)
	for i := 0; i < params.count; i++ {
		if err = uiauto.Combine("verify state after taking screenshot(s)",
			uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
				Name(screenshotNames[i])),
		)(ctx); err != nil {
			s.Fatal("Failed to verify state after taking screenshot(s): ", err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, dragAndDropHTML)
	}))
	defer server.Close()

	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer conn.Close()

	if err = conn.Navigate(ctx, server.URL); err != nil {
		s.Fatalf("Failed to navigate to %q : %s", server.URL, err)
	}

	chromeWindowFinder := nodewith.NameContaining("Google Chrome").Role(role.Window).HasClass("BrowserRootView")
	chromeLocation, err := uia.Location(ctx, chromeWindowFinder)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	uia.LeftClick(holdingspace.FindTray())(ctx)
	lastFileLocation, err := uia.Location(ctx, holdingspace.FindScreenCaptureView().Name(screenshotNames[len(screenshotNames)-1]))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}
	if err := uiauto.Combine(fmt.Sprintf("Drag and drop %d screenshots(s) from Holding Space to Chrome Browser", params.count),
		kb.AccelPressAction("Ctrl"),
		func(ctx context.Context) error {
			for _, fileName := range screenshotNames {
				if err := uia.LeftClick(holdingspace.FindScreenCaptureView().Name(fileName))(ctx); err != nil {
					s.Fatalf("Failed to select %q: %s", fileName, err)
				}
			}
			return err
		},
		kb.AccelReleaseAction("Ctrl"),
		mouse.Drag(tconn, lastFileLocation.CenterPoint(), chromeLocation.CenterPoint(), time.Second),
		uia.Gone(holdingspace.FindTrayBubble()),
		func(ctx context.Context) error {
			for _, screenshotName := range screenshotNames {
				if err := uia.WaitUntilExists(nodewith.Name(screenshotName))(ctx); err != nil {
					s.Fatalf("Failed to verify that screenshot %q was dropped into chrome: %s", screenshotName, err)
				}
			}
			return nil
		},
	)(ctx); err != nil {
		s.Fatalf("Failed to drag and drop %d screenshots(s) from Holding Space to Chrome Browser: %s", params.count, err)
	}
}
