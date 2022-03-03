package utils

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"regexp"
)

// startActivityOnDisplay starts an activity by calling "am start --display" on the given display ID.
// TODO(ruanc): Move this function to proper location (activity.go or Ash) once the external displays has better support.
func StartActivityOnDisplay(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, pkgName, actName string, dispID int) error {
	cmd := a.Command(ctx, "am", "start", "--display", fmt.Sprintf("%d", dispID), pkgName+"/"+actName)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	// Looking for:
	//  Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] cmp=com.example.name/.ActvityName }
	//  Error type 3
	//  Error: Activity class {com.example.name/com.example.name.ActvityName} does not exist.
	re := regexp.MustCompile(`(?m)^Error:\s*(.*)$`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) == 2 {
		testing.ContextLog(ctx, "Failed to start activity: ", groups[1])
		return errors.New("failed to start activity")
	}

	if err := ash.WaitForVisible(ctx, tconn, pkgName); err != nil {
		return errors.Wrap(err, "failed to wait for visible activity")
	}
	return nil
}
