// +build linux darwin

package utils

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

func Exec(command string, args []string, timeout time.Duration) (string, error) {
	if command == "" {
		return "", errors.New("command is nil")
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	cmd := exec.Command(command, args...)
	sb := bytes.NewBufferString("")
	cmd.Stdout = sb
	cmd.Stderr = sb
	var err error
	isDone := false
	go func() {
		err = cmd.Run()
		isDone = true
		cancelFunc()
	}()
	for {
		select {
		case <-ctx.Done():
			if !isDone {
				err = errors.New("timeout")
			}
			return sb.String(), err
		}
	}
	return sb.String(), err
}
