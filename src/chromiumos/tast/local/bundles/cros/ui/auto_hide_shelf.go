package ui

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoHideShelf,
		Desc:         "Checks the UI behavior of the shelf in auto hide mode",
		Contacts:     []string{"andrewxu@chromium.org", "newcomer@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
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
			var behavior = "hidden";
			chrome.autotestPrivate.setShelfAutoHideBehavior(%s, behavior, function() {
				  if(chrome.runtime.lastError) {
						reject(new Error(chrome.runtime.lastError.message));
						return;
					}
					resolve();
				});
	})
`, displayID)

	if err := tconn.EvalPromise(ctx, SetShelfAutoHideScript, &err); err != nil {
		s.Fatal("Failed to set auto hide shelf mode: ", err)
	}

}
