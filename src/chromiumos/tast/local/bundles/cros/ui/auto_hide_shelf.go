package ui

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoHideShelf,
		Desc:         "Checks the UI behavior of the shelf in auto hide mode",
		Contacts:     []string{"andrewxu@chromium.org", "newcomer@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// AutoHideShelf should have desired behavior.
func AutoHideShelf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	doesShelfExist := 0
	GetShelfExistence :=
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(function(rootNode){
				  shelf_view = rootNode.find({attributes:{role:'toolbar', name:'Shelf'}});
					resolve(shelf_view.location['height']);
			});
		})`

	if err := tconn.EvalPromise(ctx, GetShelfExistence, &doesShelfExist); err != nil {
		s.Fatal("Failed to query the state of shelf: ", err)
	}

	testing.ContextLog(ctx, "height 1: "+strconv.Itoa(doesShelfExist))

	var displayID string
	displayID = "-1"

	GetDisplayIDScript :=
		`new Promise((resolve, reject) => {
				chrome.system.display.getInfo(info => {
					var l = info.length;
					for (var i = 0; i < l; i++) {
						if (info[i].isPrimary == true)
						  resolve(info[i].id);
					}
				});
			})
		`
	if err := tconn.EvalPromise(ctx, GetDisplayIDScript, &displayID); err != nil {
		s.Fatal("Failed to get display id: ", err)
	}

	// testing.ContextLog(ctx, displayID)

	SetShelfAutoHideScript := fmt.Sprintf(
		`new Promise((resolve, reject) => {
					chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
						  if(chrome.runtime.lastError) {
								reject(new Error("!!! :" + chrome.runtime.lastError.message));
								return;
							}
							resolve();
						});
			})
		`, displayID, "always")

	// testing.ContextLog(ctx, "code: "+SetShelfAutoHideScript)

	if err := tconn.EvalPromise(ctx, SetShelfAutoHideScript, nil); err != nil {
		s.Fatal("Failed to set auto hide shelf mode: ", err)
	}

	if _, err = cr.NewConn(ctx, "https://www.google.com/"); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	doesShelfExist = 0

	if err := tconn.EvalPromise(ctx, GetShelfExistence, &doesShelfExist); err != nil {
		s.Fatal("Failed to query the state of shelf: ", err)
	}

	testing.ContextLog(ctx, "height 2: "+strconv.Itoa(doesShelfExist))

	//////////////////////////// Gesture Drag Shelf Upward ///////////////////////

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create TouchEventWriter")
	}
	defer stw.Close()

	var displayInfo display.Info
	FetchDisplayInfo := `new Promise((resolve, reject) => {
		chrome.system.display.getInfo(infos => {
			var infoLength = infos.length;
			for(var info_idx = 0; info_idx < infoLength; info_idx++) {
				if(infos[info_idx].isPrimary == false)
					continue;
				resolve(infos[info_idx]);
				}
			}
			reject(new Error("fail to find the selected mode"));
		});
	})
	`

	if err := tconn.EvalPromise(ctx, FetchDisplayInfo, &displayInfo); err != nil {
		s.Fatal("Failed to fetch the current display mode: ", err)
	}

	var currentMode display.DisplayMode
	displayModes := displayInfo["modes"]
	for _, element := range displayModes {
		if element["isSelected"] == true {
			currentMode = element
		}
	}

	/*
		dispW := displayMode.WidthInNativePixels
		dispH := displayMode.HeightInNativePixels
		pixelToTuxelX := float64(tsw.Width()) / float64(dispW)
		pixelToTuxelY := float64(tsw.Height()) / float64(dispH)
	*/

	//////////////////////////////// Show Context Menu ///////////////////////////

	ShowShelfContextMenu := `new Promise((resolve, reject) => {
		  chrome.automation.getDesktop(rootNode => {
				shelf_view = rootNode.find({attributes:{role:'toolbar', name:'Shelf'}});
				shelf_view.performStandardAction(chrome.automation.ActionType.SHOW_CONTEXT_MENU);
				resolve();
			});
		})`

	if err := tconn.EvalPromise(ctx, ShowShelfContextMenu, nil); err != nil {
		s.Fatal("Failed to show the context menu: ", err)
	}
}
