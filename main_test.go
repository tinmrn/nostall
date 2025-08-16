package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/stretchr/testify/assert"

	"os/exec"
	"testing"
)

func TestSuccess(t *testing.T) {
	t.Parallel()
	expect(t, []string{`ping`, `-c`, `2`, `127.0.0.1`}, expectParams{
		ExitCode:    0,
		StdoutMatch: regexp.MustCompile(`packets transmitted`),
	})
}

func TestFailStart(t *testing.T) {
	t.Parallel()
	expect(t, []string{`hahahahahaha`}, expectParams{
		ExitCode:    1,
		StderrMatch: regexp.MustCompile(`executable file not found in`),
	})
}

func TestFailRun(t *testing.T) {
	t.Parallel()
	expect(t, []string{`ping`, `hahahahahaha`}, expectParams{
		ExitCode:    2,
		StderrMatch: regexp.MustCompile(`Name or service not known|Temporary failure in name resolution`),
	})
}

func TestRetryFail(t *testing.T) {
	t.Parallel()
	expect(t, []string{`-wait`, `1s`, `-wait-retry`, `1s`, `-max-tries`, `3`, `ping`, `-c`, `4`, `-i`, `2`, `127.0.0.1`}, expectParams{
		ExitCode:    255,
		StdoutMatch: regexp.MustCompile(`icmp_seq`),
		StderrMatch: regexp.MustCompile(`(?s)waiting 1s for restart #2.+killing`),
	})
}

func TestRetrySuccess(t *testing.T) {
	t.Parallel()

	curEpoch := time.Now().Unix()
	cmd := fmt.Sprintf(`sleep $((%d - $(date +%%s)))`, curEpoch+10)

	expect(t, []string{`-wait`, `2s`, `-wait-retry`, `1s`, `-max-tries`, `20`, `bash`, `-c`, cmd}, expectParams{
		ExitCode:    0,
		StderrMatch: regexp.MustCompile(`waiting 1s for restart #3`),
	})
}

type expectParams struct {
	ExitCode    int
	StdoutMatch *regexp.Regexp
	StderrMatch *regexp.Regexp
}

func expect(t *testing.T, args []string, p expectParams) {
	stdOut, stdErr, exitCode := runCmd(args...)
	assert.Equalf(t, p.ExitCode, exitCode, `exit code of %v - stdout %q, stderr %q`, args, stdOut, stdErr)
	if p.StdoutMatch != nil {
		assert.Regexpf(t, p.StdoutMatch, stdOut, `StdoutMatch of %v - exit code %d, stdout %q, stderr %q`, args, exitCode, stdOut, stdErr)
	}
	if p.StderrMatch != nil {
		assert.Regexpf(t, p.StderrMatch, stdErr, `StderrMatch of %v - exit code %d, stdout %q, stderr %q`, args, exitCode, stdOut, stdErr)
	}
}

func runCmd(args ...string) (stdOut string, stdErr string, exitCode int) {
	cmd := exec.Command(`.bin/nostall`, args...)
	//log.Printf(`running %s`, cmd)
	var stdOutBuf bytes.Buffer
	var stdErrBuf bytes.Buffer
	cmd.Stdout = &stdOutBuf
	cmd.Stderr = &stdErrBuf

	cmdErr := cmd.Run()
	stdOut = stdOutBuf.String()
	stdErr = stdErrBuf.String()

	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			panic(fmt.Errorf(`%s returned %+v`, cmd, cmdErr))
		}
	}

	return
}
