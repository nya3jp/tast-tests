package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

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

	const (
		html        = "<!DOCTYPE html><input id='text' type='text' autofocus>"
		elementExpr = "document.getElementById('text')"
		valueExpr   = elementExpr + ".value"
	)
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

	s.Log("Waiting for focus")
	if err := conn.WaitForExpr(ctx, elementExpr+" === document.activeElement"); err != nil {
		s.Fatal("Failed waiting for focus: ", err)
	}

	s.Log("Finding and opening keyboard device")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	const expected = "Hello, world!"
	s.Logf("Injecting keyboard events for %q", expected)
	if err = ew.Type(expected); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	s.Log("Waiting for text")
	if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length === %d", valueExpr, len(expected))); err != nil {
		s.Fatal("Failed to wait for text: ", err)
	}
	var actual string
	if err := conn.Eval(ctx, valueExpr, &actual); err != nil {
		s.Fatal("Failed to get text: ", err)
	}
	s.Logf("Got text %q", actual)

	if actual != expected {
		s.Errorf("Got text %q; typed %q (non-QWERTY layout or Caps Lock?)", actual, expected)
	}
}
