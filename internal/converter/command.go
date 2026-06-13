package converter

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	if strings.TrimSpace(command.Name) == "" {
		return nil, fmt.Errorf("command name is required")
	}

	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return output.Bytes(), fmt.Errorf("run %s: %w: %s", commandString(command), err, strings.TrimSpace(output.String()))
	}

	return output.Bytes(), nil
}

func commandString(command Command) string {
	parts := make([]string, 0, len(command.Args)+1)
	parts = append(parts, command.Name)
	parts = append(parts, command.Args...)

	return strings.Join(parts, " ")
}
