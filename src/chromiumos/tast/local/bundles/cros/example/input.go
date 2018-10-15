package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
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

	// getText waits for expr to evaluate to a string of the given length and returns the string.
	getText := func(expr string, length int) (string, error) {
		s.Log("Waiting for text from ", expr)
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length === %d", expr, length)); err != nil {
			return "", errors.Wrapf(err, "waiting for %s failed", expr)
		}
		var actual string
		if err := conn.Eval(ctx, expr, &actual); err != nil {
			return "", errors.Wrapf(err, "evaluating %s failed", expr)
		}
		s.Logf("Got text %q from %s", actual, expr)
		return actual, nil
	}

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

	const inputText = "Hello, world!"
	s.Logf("Injecting keyboard events for %q", inputText)
	if err = ew.Type(inputText); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	// TODO(derat): The text typed above seems to sometimes not show up; try to figure out why.
	// Maybe there's a small delay within Blink between document.activeElement being updated and keyboard
	// events actually being directed to the element.
	if actual, err := getText(valueExpr, len(inputText)); err != nil {
		s.Error("Failed to get input text (this can be flaky): ", err)
	} else if actual != inputText {
		s.Errorf("Got input text %q; typed %q (non-QWERTY layout or Caps Lock?)", actual, inputText)
	}

	const (
		pageText = "mittens"
		dataURL  = "data:text/plain," + pageText
		bodyExpr = "document.body.innerText"
	)
	s.Logf("Navigating to %q via omnibox", dataURL)
	ew.Accel("Ctrl+L")
	for _, r := range []rune(dataURL + "\n") {
		// TODO(derat): Without sleeping between keystrokes, the omnibox seems to produce scrambled text.
		// Figure out why. Presumably there's a bug in Chrome's input stack or the omnibox code.
		time.Sleep(50 * time.Millisecond)
		ew.Type(string(r))
	}
	if actual, err := getText(bodyExpr, len(pageText)); err != nil {
		s.Error("Failed to get page text: ", err)
	} else if actual != pageText {
		s.Errorf("Got page text %q; want %q", actual, pageText)
	}
}
