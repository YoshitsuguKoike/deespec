package claudecli

import (
	"context"
	"os/exec"
	"time"
)

type Runner struct {
	Bin     string
	Timeout time.Duration
}

func (r Runner) Run(ctx context.Context, prompt string, extraArgs ...string) (string, error) {
	args := []string{"-p", "--output-format", "stream-json"}
	args = append(args, extraArgs...) // 将来拡張用
	args = append(args, prompt)

	cctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, r.Bin, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}