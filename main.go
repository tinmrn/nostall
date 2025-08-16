package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/tinmrn/nostall/writetracker"
)

var stallTime time.Duration
var waitRestart time.Duration
var maxTries int

func main() {
	err := run()
	if err != nil {
		slog.Error(fmt.Sprintf(`%v`, err))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func run() error {

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.DurationVar(&stallTime, `wait`, 1*time.Minute, `The duration of stalled output after which to restart the program.`)
	fs.DurationVar(&waitRestart, `wait-retry`, 10*time.Second, `The time to wait before restarting the program if it stalls.`)
	fs.IntVar(&maxTries, `max-tries`, 0, `Max number of tries`)

	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	args := fs.Args()

	if len(args) == 0 {
		return errors.New(`give command and parameters to run`)
	}

	cmdBase := args[0]
	cmdArgs := make([]string, 0)
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	ctx, cnc := context.WithCancel(context.Background())
	defer cnc()

	sigs := make(chan os.Signal, 10)
	signal.Notify(sigs, os.Interrupt, os.Kill)
	go func() {
		for sig := range sigs {
			slog.Info(`received signal`, `sig`, sig)
			cnc()
		}
	}()

	for try := 1; ; try++ {

		// run the iteration inside a func so we can defer the cancellation of the command context
		err = func() error {
			cmdCtx, cmdCnc := context.WithCancel(ctx)
			defer cmdCnc()

			cmd := exec.CommandContext(cmdCtx, cmdBase, cmdArgs...)

			slog.Debug(fmt.Sprintf(`running %s with max stall time %s`, cmd, stallTime))

			cmd.Cancel = func() error {
				pgid, pgidErr := syscall.Getpgid(cmd.Process.Pid)
				if pgidErr != nil {
					return fmt.Errorf(`error getting pgid: %+v`, pgidErr)
				}
				killErr := syscall.Kill(-pgid, syscall.SIGKILL)
				if killErr != nil {
					return fmt.Errorf(`error killing pgid %d: %+v`, -pgid, killErr)
				}
				return nil
			}

			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			stdOut := writetracker.New(os.Stdout, true)
			stdErr := writetracker.New(os.Stderr, true)

			cmd.Stdout = stdOut
			cmd.Stderr = stdErr
			cmd.Stdin = os.Stdin

			err = cmd.Start()
			// If Start fails, return immediately without restarting, because that probably means the cmd will never run.
			if err != nil {
				return err
			}

			cmdDoneCtx, cmdDoneCnc := context.WithCancel(context.Background())

			go monitor(cmdDoneCtx, cmdCnc, cmd, stdOut, stdErr)

			cmdErr := cmd.Wait()
			cmdDoneCnc()

			if cmdErr == nil {
				slog.Debug(fmt.Sprintf(`%d exits`, cmd.Process.Pid))
				return nil
			}

			slog.Info(fmt.Sprintf(`%d exits: %v`, cmd.Process.Pid, cmdErr))

			if maxTries > 0 && try >= maxTries {
				return cmdErr
			}

			if isCtxClosed(cmdCtx) {
				return retryError{err: cmdErr}
			}

			return cmdErr
		}()
		if err != nil {
			var re retryError
			if errors.As(err, &re) {
				slog.Info(fmt.Sprintf(`waiting %s for restart #%d...`, waitRestart, try))
				err = sleepCtx(ctx, waitRestart)
				if err != nil {
					return fmt.Errorf(`sleep cancelled: %v`, err)
				}
				continue
			}
			return err
		}
		break
	}

	return nil
}

func isCtxClosed(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func sleepCtx(ctx context.Context, sleep time.Duration) error {
	t := time.NewTimer(sleep)
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func monitor(ctx context.Context, kill func(), cmd *exec.Cmd, stdOut *writetracker.WriteTracker, stdErr *writetracker.WriteTracker) {

	checkInterval := 1 * time.Second
	if stallTime < checkInterval {
		checkInterval = stallTime
	}

	t := time.NewTicker(checkInterval)

	for {
		select {
		case <-t.C:
			lastOutputAgo := time.Since(time.UnixMilli(max(stdOut.GetLastUnixMilli(), stdErr.GetLastUnixMilli())))
			if lastOutputAgo > stallTime {
				slog.Error(fmt.Sprintf(`last output was %s ago, killing %d`, lastOutputAgo.Truncate(time.Second), cmd.Process.Pid))

				kill()
			}
		case <-ctx.Done():
			return
		}
	}

}

type retryError struct {
	err error
}

func (r retryError) Error() string {
	return r.err.Error()
}
