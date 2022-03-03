package utils

import (
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"fmt"
)

// suspend then reconnect chromebook for 15s
func SuspendChromebook(ctx context.Context, s *testing.State, cr *chrome.Chrome) (*chrome.TestConn, error) {

	s.Logf("Suspend 15s then reconnect chromebook")

	command := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s %s", "powerd_dbus_suspend", "--suspend_for_sec=15"))

	if err := command.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "Failed to execute powerd_dbus_suspend command: ")
	}

	// reconnect chrome
	if err := cr.Reconnect(ctx); err != nil {
		return nil, errors.Wrap(err, "Failed to reconnect to chrome: ")
	}

	// re-build API connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Test API connection: ")
	}

	return tconn, nil
}

// notice: if chromebook was powered off, then cause SSH lost
func PoweroffChromebook(ctx context.Context, s *testing.State) error {

	s.Logf("Power off chromebook")

	err := testexec.CommandContext(ctx, "shutdown", "-P", "now").Run(testexec.DumpLogOnError)
	// err := testexec.CommandContext(ctx, "shutdown", "-P", "now").Start()
	if err != nil {
		return errors.Wrapf(err, "Failed to execute power off chromebook: ")
	}

	return nil
}
