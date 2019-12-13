package sender

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
)

// SetUp sets up environment suitable for crash_sender testing.
// cr is a logged-in chrome session. TearDown must be called later to clean up.
func SetUp(ctx context.Context, cr *chrome.Chrome) (crashDir string, retErr error) {
	defer func() {
		if retErr != nil {
			TearDown()
		}
	}()

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		return "", err
	}

	if err := EnableMock(MockSuccess); err != nil {
		return "", err
	}

	if err := ResetSentReports(); err != nil {
		return "", err
	}

	// Create a temporary crash dir to use with crash_sender.
	crashDir, err := ioutil.TempDir("", "crash.")
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary crash dir")
	}
	return crashDir, err
}

// TearDown cleans up environment set up by SetUp.
func TearDown() error {
	var firstErr error
	if err := DisableMock(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := crash.TearDownCrashTest(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
