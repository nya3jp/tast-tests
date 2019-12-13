package sender

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/syslog"
)

func TestParseCrashSenderLogs(t *testing.T) {
	now := time.Date(2019, 12, 11, 14, 41, 28, 0, time.UTC)
	const log = `Checking metadata: /var/spool/crash/fake.1.2.3.meta
Checking metadata: /var/spool/crash/fake.2.3.4.meta
Evaluating crash report: /var/spool/crash/fake.1.2.3.meta
Scheduled to send in 198s
Current send rate: 8 sends and 10828 bytes/24hrs
Sending crash:
  Metadata: /var/spool/crash/fake.1.2.3.meta (minidump)
  Payload: /var/spool/crash/fake.1.2.3.dmp
  Version: my_ver
  Product: ChromeOS
  URL: https://clients2.google.com/cr/report
  Board: betty-pi-arc
  HWClass: undefined
  Exec name: fake
Mocking successful send
Successfully sent crash /var/spool/crash/fake.1.2.3.meta and removing.
Evaluating crash report: /var/spool/crash/fake.2.3.4.meta
Scheduled to send in 451s
Current send rate: 9 sends and 12180 bytes/24hrs
Sending crash:
  Metadata: /var/spool/crash/fake.2.3.4.meta (minidump)
  Payload: /var/spool/crash/fake.2.3.4.dmp
  Version: my_ver
  Product: ChromeOS
  URL: https://clients2.google.com/cr/report
  Board: betty-pi-arc
  HWClass: undefined
  Exec name: fake
Mocking successful send
Successfully sent crash /var/spool/crash/fake.2.3.4.meta and removing.
crash_sender done. (mock)`
	var es []*syslog.Entry
	for _, line := range strings.Split(log, "\n") {
		es = append(es, &syslog.Entry{Timestamp: now, Content: line})
	}

	got, err := parseLogs(es)
	if err != nil {
		t.Fatal("parseLogs failed: ", err)
	}

	want := []*SendResult{
		{
			Schedule: now.Add(198 * time.Second),
			Success:  true,
			Data: SendData{
				MetadataPath: "/var/spool/crash/fake.1.2.3.meta",
				PayloadPath:  "/var/spool/crash/fake.1.2.3.dmp",
				PayloadKind:  "minidump",
				Product:      "ChromeOS",
				Version:      "my_ver",
				Board:        "betty-pi-arc",
				HWClass:      "undefined",
				Executable:   "fake",
			},
		},
		{
			Schedule: now.Add(451 * time.Second),
			Success:  true,
			Data: SendData{
				MetadataPath: "/var/spool/crash/fake.2.3.4.meta",
				PayloadPath:  "/var/spool/crash/fake.2.3.4.dmp",
				PayloadKind:  "minidump",
				Product:      "ChromeOS",
				Version:      "my_ver",
				Board:        "betty-pi-arc",
				HWClass:      "undefined",
				Executable:   "fake",
			},
		},
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("ParseCrashSenderLogs returned unexpected results (-got +want):\n%s", diff)
	}
}
