package arc

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/testing"
)

// WaitSystemEvent waits for ARC system event to be emitted.
// This function will return when the given event is emitted or ctx's deadline expires.
func WaitSystemEvent(ctx context.Context, name string) error {
	cmd := CommandContext(ctx, "logcat", "-b", "events", "*:S", "arc_system_event")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	testing.ContextLogf(ctx, "Waiting for ARC system event %v", name)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		testing.ContextLog(ctx, line)
		if strings.HasSuffix(line, " "+name) {
			return nil
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("ARC system event %v never seen", name)
}
