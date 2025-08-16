# nostall
Run a command and kill+restart it if its output stalls for a configurable amount of time.

## why
Because I need to rsync a couple of TB of data and rsync keeps stalling, ignoring its `--timeout` flag.

## usage

```
Usage: nostall [params] cmd [args...]

  -max-tries int
        Max number of tries
  -wait duration
        The duration of stalled output after which to restart the program. (default 1m0s)
  -wait-retry duration
        The time to wait before restarting the program if it stalls. (default 10s)
```
