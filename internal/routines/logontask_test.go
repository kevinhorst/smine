package routines

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogonTaskXML(t *testing.T) {
	xml := LogonTaskXML(`--addr 127.0.0.1:6101 --log-file "C:\logs\heartbeat.log" serve`, `C:\repo\bin\heartbeat.exe`, `C:\repo`)

	assert.Contains(t, xml, "<LogonTrigger><Enabled>true</Enabled></LogonTrigger>")
	assert.Contains(t, xml, `<Command>C:\repo\bin\heartbeat.exe</Command>`)
	assert.Contains(t, xml, `<Arguments>--addr 127.0.0.1:6101 --log-file &quot;C:\logs\heartbeat.log&quot; serve</Arguments>`)
	assert.Contains(t, xml, `<WorkingDirectory>C:\repo</WorkingDirectory>`)
	assert.Contains(t, xml, "<RestartOnFailure>")
}
