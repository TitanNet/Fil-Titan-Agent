package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/creack/pty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

var (
	wssshCheckInterval = 2 * time.Minute
	wssshRetryInterval = 15 * time.Second
)

func (c *Controller) checkConnectable(ctx context.Context) (string, error) {

	url := fmt.Sprintf("%s%s?node_id=%s", c.args.ServerURL, "/config/ssh", c.Config.AgentID)

	ctx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("wsssh status code: %d, msg: %s, url: %s", resp.StatusCode, string(body), url)
	}

	if resp.StatusCode == http.StatusForbidden {
		return "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Controller) handleWsssh(ctx context.Context) error {
	timer := time.NewTicker(wssshCheckInterval)
	defer timer.Stop()

	connCtx, cancel := context.WithCancel(ctx)

	connStr, _ := c.checkConnectable(ctx)
	if connStr != "" {
		go logFuncError(c.connectAndServe, connCtx, connStr)
	}

	for {
		select {
		case <-timer.C:
			newConnstr, err := c.checkConnectable(ctx)
			if err != nil {
				log.Errorf("handleWsssh.checkConnectable: %v", err)
				continue
			}
			if newConnstr != connStr {
				log.Infof("handleWsssh: conn string changed: %s -> %s", connStr, newConnstr)
				connStr = newConnstr
				cancel()

				if connStr != "" {
					connCtx, cancel = context.WithCancel(ctx)
					go logFuncError(c.connectAndServe, connCtx, connStr)
				}
			}

		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}
}

func logFuncError(fn func(context.Context, string) error, ctx context.Context, conn string) {
	if err := fn(ctx, conn); err != nil {
		log.Error(err)
	}
}

func (c *Controller) connectAndServe(ctx context.Context, conn string) error {
	for {
		select {
		case <-ctx.Done():
			log.Warnf("connectAndServe context canceled: %v", ctx.Err())
			return ctx.Err()
		default:
		}

		u := url.URL{
			Scheme:   "ws",
			Host:     conn,
			Path:     "/agent/ws",
			RawQuery: fmt.Sprintf("nodeId=%s", c.Config.AgentID),
		}

		log.Info("Connecting to ", u.String())

		ws, err := websocket.Dial(u.String(), "", c.args.ServerURL)
		if err != nil {
			log.Error("Retrying in 5s...", err)
			time.Sleep(wssshRetryInterval)
			continue
		}

		log.Info("Connected to server")

		switch runtime.GOOS {
		case "windows":
			runWindowsShell(ctx, ws)
		case "linux", "darwin":
			runUnixShell(ctx, ws)
		case "android":
			runAndroidShell(ctx, ws)
		default:
			time.Sleep(wssshCheckInterval)
			log.Error("Unsupported platform:", runtime.GOOS)
		}

		ws.Close()
		log.Error("Connection closed, retrying in 5s...")
		time.Sleep(wssshRetryInterval)
	}
}

func runUnixShell(ctx context.Context, ws *websocket.Conn) {
	cmd := exec.CommandContext(ctx, "bash")
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		log.Error("PTY error:", err)
		return
	}

	errChan := make(chan error)
	go func() {
		_, err := io.Copy(ptyFile, ws)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(ws, ptyFile)
		errChan <- err
	}()

	go func() {
		<-ctx.Done()
		_ = cmd.Process.Kill()
	}()

	log.Errorf("run command error: %v", <-errChan)

	_ = cmd.Process.Kill()
}

func runAndroidShell(ctx context.Context, ws *websocket.Conn) {
	cmd := exec.CommandContext(ctx, "/system/bin/sh")
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		log.Error("PTY error:", err)
		return
	}

	errChan := make(chan error)
	go func() {
		_, err := io.Copy(ptyFile, ws)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(ws, ptyFile)
		errChan <- err
	}()

	go func() {
		<-ctx.Done()
		_ = cmd.Process.Kill()
	}()

	log.Errorf("run command error: %v", <-errChan)

	_ = cmd.Process.Kill()
}

func runWindowsShell(ctx context.Context, ws *websocket.Conn) {
	cmd := exec.CommandContext(ctx, "powershell.exe")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("StdinPipe error:", err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("StdoutPipe error:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("StderrPipe error:", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Error("Command start failed:", err)
		return
	}

	errChan := make(chan error, 3)

	go func() {
		_, err := io.Copy(stdin, ws)
		errChan <- fmt.Errorf("stdin copy error: %v", err)
	}()

	go func() {
		_, err := io.Copy(ws, stdout)
		errChan <- fmt.Errorf("stdout copy error: %v", err)
	}()

	go func() {
		_, err := io.Copy(ws, stderr)
		errChan <- fmt.Errorf("stderr copy error: %v", err)
	}()

	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	log.Error("run command error:", <-errChan)

	cmd.Process.Kill()
	cmd.Wait()

}
