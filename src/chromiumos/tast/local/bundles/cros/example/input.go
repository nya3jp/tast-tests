package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Input,
		Desc: "Demonstrates injecting keyboard events",
		// TODO(derat): Remove "disabled" if/when there's a way to depend on an internal keyboard.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Input(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	const html = "<html><body><input id='text' type='text' autofocus></body></html>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	s.Log("Loading input page")
	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	s.Log("Injecting keyboard events")
	ew := input.Keyboard(ctx)
	for _, code := range []input.EventCode{input.KEY_H, input.KEY_E, input.KEY_L, input.KEY_L, input.KEY_O} {
		ew.Event(time.Now(), input.EV_KEY, code, 1)
		ew.Sync(time.Now())
		ew.Event(time.Now(), input.EV_KEY, code, 0)
		ew.Sync(time.Now())
	}
	if err := ew.Close(); err != nil {
		s.Fatal("Failed to inject events: ", err)
	}

	const (
		expr     = "document.getElementById('text').value"
		expected = "hello"
	)
	s.Log("Waiting for text")
	if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length == %d", expr, len(expected))); err != nil {
		s.Fatal("Failed to wait for text: ", err)
	}
	var actual string
	if err := conn.Eval(ctx, expr, &actual); err != nil {
		s.Fatal("Failed to get text: ", err)
	}
	s.Logf("Got text %q", actual)
	if actual != expected && actual != "d.nnr" { // silly hack: also support text for dvorak layout
		s.Errorf("Got text %q; typed %q", actual, expected)
	}
}
