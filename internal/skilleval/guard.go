package skilleval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// initGuard cancels on the first init record, so an unknown --model costs one line, not a whole arm.
type initGuard struct {
	arm       *ManifestArm
	cancel    context.CancelFunc
	isChecked bool
	out       *os.File
	pending   []byte
	problems  []string
	required  []string
}

func (g *initGuard) Write(data []byte) (int, error) {
	written, err := g.out.Write(data)
	if err != nil {
		return written, fmt.Errorf("initGuard.Write: %w", err)
	}

	if g.isChecked {
		return written, nil
	}

	g.pending = append(g.pending, data...)
	for !g.isChecked {
		cut := bytes.IndexByte(g.pending, '\n')
		if cut < 0 {
			break
		}

		line := g.pending[:cut]
		g.pending = g.pending[cut+1:]
		g.inspect(line)
	}
	if g.isChecked {
		g.pending = nil
	}
	return written, nil
}

func (g *initGuard) inspect(line []byte) {
	var record streamRecord
	isInit := json.Unmarshal(line, &record) == nil && record.Type == recordSystem && record.Subtype == subtypeInit
	if !isInit {
		return
	}

	g.isChecked = true
	var initRecord InitRecord
	if err := json.Unmarshal(line, &initRecord); err != nil {
		return
	}

	g.problems = variantProblems(g.arm, &initRecord, g.required)
	if len(g.problems) > 0 {
		g.cancel()
	}
}
