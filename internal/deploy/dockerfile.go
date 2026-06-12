package deploy

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PortDetector interface {
	DetectPort(ctx context.Context, sourceDir string) (int64, bool, error)
}

type PortDetectorFunc func(ctx context.Context, sourceDir string) (int64, bool, error)

func (fn PortDetectorFunc) DetectPort(ctx context.Context, sourceDir string) (int64, bool, error) {
	return fn(ctx, sourceDir)
}

type DockerfilePortDetector struct{}

func (DockerfilePortDetector) DetectPort(ctx context.Context, sourceDir string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}

	file, err := os.Open(filepath.Join(sourceDir, "Dockerfile"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		port, ok := exposePort(scanner.Text())
		if ok {
			return port, true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, false, err
	}
	return 0, false, nil
}

func exposePort(line string) (int64, bool) {
	line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.EqualFold(fields[0], "EXPOSE") {
		return 0, false
	}

	for _, field := range fields[1:] {
		portText := strings.SplitN(field, "/", 2)[0]
		port, err := strconv.ParseInt(portText, 10, 64)
		if err == nil && port >= 1 && port <= 65535 {
			return port, true
		}
	}
	return 0, false
}
