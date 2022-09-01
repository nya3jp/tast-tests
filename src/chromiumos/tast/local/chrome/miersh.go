package chrome

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/login"
	"chromiumos/tast/local/chrome/jslog"
	"context"
)

func MiershLogIn(ctx context.Context, cr *Chrome, opts ...Option) (retErr error) {
	cfg, err := config.NewConfig(opts)
	if err != nil {
		return errors.Wrap(err, "failed to process options")
	}

	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()

	// sess, err := driver.NewSession(ctx, ashproc.ExecPath, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
	// if err != nil {
	// 	return errors.Wrapf(err, "failed to establish connection to Chrome Debugging Protocol with debugging port path=%q", cdputil.DebuggingPortPath)
	// }
	// defer func() {
	// 	if retErr != nil {
	// 		sess.Close(ctx)
	// 	}
	// }()

	existingSession := cr.sess

	if err := login.LogIn(ctx, cfg, existingSession); err == login.ErrNeedNewSession {
		// Restart session.
		newSess, err := driver.NewSession(ctx, ashproc.ExecPath, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
		if err != nil {
			return errors.Wrap(err, "failed to reconnect to restarted session")
		}
		existingSession.Close(ctx)
		cr.sess = newSess
	} else if err != nil {
		return errors.Wrap(err, "login failed")
	}

	return nil
}

// Doesn't actually fix anything =(
func FixChromeSession(ctx context.Context, cr *Chrome) (retErr error) {
	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()

	newSess, err := driver.NewSession(ctx, ashproc.ExecPath, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
	if err != nil {
		return errors.Wrap(err, "failed to reconnect to restarted session")
	}
	cr.sess = newSess
	return nil
}
