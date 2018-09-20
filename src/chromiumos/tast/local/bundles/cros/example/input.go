package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

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

	const html = "<!DOCTYPE html><input id='text' type='text' autofocus>"
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
	if err := conn.WaitForExpr(ctx, `document.getElementById('text') === document.activeElement`); err != nil {
		s.Fatal("Failed waiting for focus: ", err)
	}

	s.Log("Finding and opening keyboard device")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	// TODO(derat): Replace all of this once the input package exposes friendly
	// methods for injecting sequences of events.
	s.Log("Injecting keyboard events")
	for _, code := range []input.EventCode{input.KEY_H, input.KEY_E, input.KEY_L, input.KEY_L, input.KEY_O} {
		if err := ew.Event(input.EV_KEY, code, 1); err != nil {
			s.Fatalf("Failed to write key down event for 0x%x: %v", code, err)
		}
		if err := ew.Sync(); err != nil {
			s.Fatalf("Failed to write key down sync for 0x%x: %v", code, err)
		}
		if err := ew.Event(input.EV_KEY, code, 0); err != nil {
			s.Fatalf("Failed to write key up event for 0x%x: %v", code, err)
		}
		if err := ew.Sync(); err != nil {
			s.Fatalf("Failed to write key up sync for 0x%x: %v", code, err)
		}
	}

	const (
		expr     = "document.getElementById('text').value"
		expected = "hello"
	)
	s.Log("Waiting for text")
	if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length === %d", expr, len(expected))); err != nil {
		s.Fatal("Failed to wait for text: ", err)
	}
	var actual string
	if err := conn.Eval(ctx, expr, &actual); err != nil {
		s.Fatal("Failed to get text: ", err)
	}
	s.Logf("Got text %q", actual)

	// Support Caps Lock and Dvorak.
	if lower := strings.ToLower(actual); lower != expected && lower != "d.nnr" {
		s.Errorf("Got text %q; typed %q", actual, expected)
	}
}
