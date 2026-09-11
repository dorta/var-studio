package supportlog

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const maxLogSize = 8 << 20

func Start(
	ctx context.Context,
	directory, bootID, device string,
) error {
	if device == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	input, err := os.Open(device)
	if err != nil {
		return err
	}
	if err := syscall.SetNonblock(int(input.Fd()), true); err != nil {
		input.Close()
		return err
	}
	if os.Getuid() == 0 {
		if err := syscall.Setgroups([]int{}); err != nil {
			input.Close()
			return err
		}
		if err := syscall.Setgid(65532); err != nil {
			input.Close()
			return err
		}
		if err := syscall.Setuid(65532); err != nil {
			input.Close()
			return err
		}
	}
	logPath := filepath.Join(directory, "kernel.log")
	bootPath := filepath.Join(directory, "kernel-boot-id")
	previousBoot, _ := os.ReadFile(bootPath)
	if strings.TrimSpace(string(previousBoot)) != bootID {
		if err := os.WriteFile(logPath, nil, 0o640); err != nil {
			return err
		}
		if err := os.WriteFile(bootPath, []byte(bootID+"\n"), 0o640); err != nil {
			return err
		}
	}
	lastSequence := lastSequence(logPath)
	output, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		input.Close()
		return err
	}
	go spool(ctx, input, output, logPath, lastSequence)
	return nil
}

func spool(
	ctx context.Context,
	input, output *os.File,
	logPath string,
	last uint64,
) {
	defer input.Close()
	defer output.Close()
	buffer := make([]byte, 64<<10)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		count, err := input.Read(buffer)
		if count > 0 {
			for _, line := range strings.Split(string(buffer[:count]), "\n") {
				sequence, ok := sequence(line)
				if line == "" || (ok && sequence <= last) {
					continue
				}
				if ok {
					last = sequence
				}
				_, _ = io.WriteString(output, line+"\n")
			}
			if info, statErr := output.Stat(); statErr == nil &&
				info.Size() > maxLogSize {
				output.Close()
				_ = os.Remove(logPath + ".1")
				_ = os.Rename(logPath, logPath+".1")
				output, _ = os.OpenFile(
					logPath,
					os.O_CREATE|os.O_APPEND|os.O_WRONLY,
					0o640,
				)
			}
		}
		if err != nil && !errors.Is(err, syscall.EAGAIN) &&
			!errors.Is(err, syscall.EWOULDBLOCK) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func lastSequence(path string) uint64 {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	var last uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if value, ok := sequence(scanner.Text()); ok &&
			value > last {
			last = value
		}
	}
	return last
}

func sequence(line string) (uint64, bool) {
	fields := strings.SplitN(line, ",", 3)
	if len(fields) < 2 {
		return 0, false
	}
	value, err := strconv.ParseUint(fields[1], 10, 64)
	return value, err == nil
}
