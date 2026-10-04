package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"time"
)

var quickTunnelURL = regexp.MustCompile("https://[a-zA-Z0-9-]+\\.trycloudflare\\.com")

type Result struct {
	URL string
}

func CloudflaredPath() (string, error) {
	path, err := exec.LookPath("cloudflared")
	if err != nil {
		return "", errors.New("cloudflared not found; install it or run cbx serve --local")
	}
	return path, nil
}

func StartCloudflareQuick(ctx context.Context, port int) (Result, error) {
	path, err := CloudflaredPath()
	if err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, path,
		"tunnel",
		"--url", fmt.Sprintf("http://127.0.0.1:%d", port),
		"--no-autoupdate",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}

	urlCh := make(chan string, 1)
	scan := func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if match := quickTunnelURL.FindString(scanner.Text()); match != "" {
				select {
				case urlCh <- match:
				default:
				}
			}
		}
	}
	go scan(stdout)
	go scan(stderr)

	exitCh := make(chan error, 1)
	go func() { exitCh <- cmd.Wait() }()

	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()

	select {
	case url := <-urlCh:
		return Result{URL: url}, nil
	case err := <-exitCh:
		if err == nil {
			err = errors.New("cloudflared exited before returning a public URL")
		}
		return Result{}, err
	case <-timer.C:
		_ = cmd.Process.Kill()
		return Result{}, errors.New("timed out waiting for Cloudflare Quick Tunnel URL")
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}
