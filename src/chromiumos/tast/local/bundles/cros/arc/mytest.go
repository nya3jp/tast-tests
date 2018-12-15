package arc

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Mytest,
		Desc:         "Checks that Android boots",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

// Mytest is a function
func Mytest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	testctx := &arc.TestContext{}
	cwd, err := os.Getwd()
	if err != nil {
		s.Log(err)
		return
	}
	err = testctx.Init(ctx, cwd+"test.json")
	if err != nil {
		s.Log(err)
		return
	}
	testctx.Setup()
	testctx.Run()
	testctx.Cleanup()
}
