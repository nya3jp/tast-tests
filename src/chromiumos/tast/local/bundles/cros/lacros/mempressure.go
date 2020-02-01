package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/bundles/cros/platform/chromewpr"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Mempressure,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		// Future versions of this test will have other parameters corresponding to the source of the linux-chrome binary.
		Data: []string{
			mempressure.CompressibleData,
			mempressure.DormantCode,
			mempressure.WPRArchiveName,
			launcher.DataArtifact,
		},
		Pre:       launcher.StartedByData(),
		Timeout:   180 * time.Minute,
	})
}

func Mempressure(ctx context.Context, s *testing.State) {
	p := &mempressure.RunParameters{
		DormantCodePath:          s.DataPath(mempressure.DormantCode),
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
	}
	cp := &chromewpr.Params{
		WPRArchivePath: s.DataPath(mempressure.WPRArchiveName),
		// UseLiveSites: true,
	}
	w, err := chromewpr.New(ctx, cp)
	if err != nil {
		s.Fatal("Failed to start WPR: ", err)
	}

	l, err := launcher.LaunchLinuxChrome(ctx, s.PreValue().(launcher.PreData), w.HttpPort, w.HttpsPort)
	if err != nil {
		s.Fatal("Failed to launch linux-chrome")
	}
	defer l.Close(ctx)

	_, err = l.Devsess.CreateTarget(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab")
	}
	ashChrome := s.PreValue().(launcher.PreData).Chrome
	c := new(chrome.Chrome)
	c.TestExtDir = ashChrome.TestExtDir
	c.TestExtID = ashChrome.TestExtID
	c.ExtDirs = ashChrome.ExtDirs
	c.LogMaster = jslog.NewMaster()
	c.Devsess = l.Devsess
	id, err := mempressure.GetActiveTabID(ctx, c)
	if err != nil {
		s.Fatal("Failed to get tab id")
	}
	s.Log("tab id: ", id)

	defer w.Close(ctx)

	// Without a sleep here, we immediately hit an early error:
	// Error at mempressure.go:724: Cannot add initial tab from list: cannot create new renderer: cdp.Target: AttachToTarget: websocket: close 1006 (abnormal closure): unexpected EOF
	// This also seems to be correlated with the WPR flags passed to linux-chrome.
	time.Sleep(10* time.Second)

	// This runs for about 5 minutes. Then a renderer crashes, which causes the test to stall.
	mempressure.Run(ctx, s, c, p)

}
