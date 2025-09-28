package main

import (
	ahttp "agent/common/http"
	"agent/common/wallet"
	"agent/controller"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var versionCmd = &cli.Command{
	Name: "version",
	Before: func(cctx *cli.Context) error {
		return nil
	},
	Action: func(cctx *cli.Context) error {
		fmt.Println(controller.Version)
		return nil
	},
}

var testCmd = &cli.Command{
	Name: "test",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "path",
			Usage:    "--path=/path/to/luafile",
			Required: true,
			Value:    "",
		},
		&cli.IntFlag{
			Name:  "time",
			Usage: "--time 60",
			Value: 60,
		},
	},
	Before: func(cctx *cli.Context) error {
		return nil
	},

	Action: func(cctx *cli.Context) error {
		luaPath := cctx.String("path")
		controllerArgs := &controller.ConrollerArgs{WorkingDir: filepath.Dir(luaPath), RelAppsDir: ""}
		appConfig := &controller.AppConfig{AppName: "test", AppDir: "", ScriptName: filepath.Base(luaPath)}

		args := &controller.AppArguments{ControllerArgs: controllerArgs, AppConfig: appConfig}
		app, err := controller.NewApplication(args, nil)
		if err != nil {
			return err
		}

		t := cctx.Int("time")

		go func() {
			time.Sleep(time.Duration(t) * time.Second)
			app.Stop()
		}()

		app.Run()
		return nil
	},
}
var runCmd = &cli.Command{
	Name:  "run",
	Usage: "run controller",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "working-dir",
			Usage:    "--working-dir=/path/to/working/dir",
			EnvVars:  []string{"WORKING_DIR"},
			Required: true,
			Value:    "",
		},
		&cli.IntFlag{
			Name:    "script-interval",
			Usage:   "--script-interval 60",
			EnvVars: []string{"SCRIPT_INTERVAL"},
			Value:   10,
		},
		&cli.StringFlag{
			Name:     "server-url",
			Usage:    "--server-url http://localhost:8080/update/lua",
			EnvVars:  []string{"SERVER_URL"},
			Required: true,
			Value:    "http://localhost:8080/update/lua",
		},
		&cli.StringFlag{
			Name:     "web-url",
			Usage:    "--web-url http://localhost:8080",
			EnvVars:  []string{"WEB_URL"},
			Value:    "http://localhost:8080",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "key",
			Usage:    "--key xxx",
			EnvVars:  []string{"KEY"},
			Value:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "rel-apps-dir",
			Usage:   "--rel-app-dir apps",
			EnvVars: []string{"RELATIVE_APPS_DIR"},
			Value:   "apps",
		},
		&cli.StringFlag{
			Name:    "appconfigs-filename",
			Usage:   "--appconfigs-filename config.json",
			EnvVars: []string{"APPCONFIGFS_FILENAME"},
			Value:   "config.json",
		},
		&cli.StringFlag{
			Name:    "uuid",
			Usage:   "--uuid fbf600d4-8ada-11ef-9e79-c3ce2c7cb2d3",
			EnvVars: []string{"UUID"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "log-file",
			Usage:   "--log-file /path/to/logfile",
			EnvVars: []string{"LOG_FILE"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:  "channel",
			Usage: "--channel titan or painet",
		},
	},
	Before: func(cctx *cli.Context) error {
		return nil
	},
	Action: func(cctx *cli.Context) error {
		// set log file
		logFile := cctx.String("log-file")
		if len(logFile) != 0 {
			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				log.Fatalf("open file %s, failed:%s", logFile, err.Error())
			}
			defer file.Close()

			log.SetOutput(file)
			os.Stdout = file
		}

		fmt.Print(logoWindow)

		args := &controller.ConrollerArgs{
			WorkingDir:           cctx.String("working-dir"),
			ServerURL:            cctx.String("server-url"),
			ScriptUpdateInterval: cctx.Int("script-interval"),
			AppConfigsFileName:   cctx.String("appconfigs-filename"),
			RelAppsDir:           cctx.String("rel-apps-dir"),
			Channel:              cctx.String("channel"),
			WebServerUrl:         cctx.String("web-url"),
			KEY:                  cctx.String("key"),
		}

		ctr, err := controller.New(args)
		if err != nil {
			log.Fatal(err)
		}

		ctx, done := context.WithCancel(context.Background())
		sigChan := make(chan os.Signal, 2)
		go func() {
			<-sigChan
			done()
		}()

		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
		return ctr.Run(ctx)
	},
}

// stupid bind func for stupid client
// ruins agent-controller structure
// will generate agent_id and prikey instead of controller doing
// damn
var bindCmd = &cli.Command{
	Name: "bind",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "working-dir",
			Usage:   "--working-dir=/path/to/working/dir",
			EnvVars: []string{"WORKING_DIR"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:  "key",
			Usage: "--key YOUR_WEB_KEY",
			Value: "",
		},
		&cli.StringFlag{
			Name:    "web-url",
			Usage:   "--web-url http://localhost:8080",
			EnvVars: []string{"WEB_URL"},
			Value:   "",
		},
	},
	Before: func(cctx *cli.Context) error {
		return nil
	},

	Action: func(cctx *cli.Context) error {
		workingDir := cctx.String("working-dir")
		if workingDir == "" {
			return cli.Exit("--working-dir is required", -1)
		}

		key := cctx.String("key")
		if key == "" {
			return cli.Exit("--key is required", -1)
		}

		webUrl := cctx.String("web-url")
		if webUrl == "" {
			return cli.Exit("--web-url is required", -1)
		}
		webUrl = fmt.Sprintf("%s%s", webUrl, "/api/network/bind_node")

		cfg, err := controller.InitConfig(workingDir)
		if err != nil {
			return cli.Exit(err.Error(), -1)
		}

		sign, err := cfg.Wallet.Sign(wallet.DefaultKeyName, []byte(key))
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to sign key: %s", err.Error()), -1)
		}

		type BindReq struct {
			Key    string `json:"key"`
			NodeID string `json:"node_id"`
			Sign   string `json:"sign"`
		}

		bindReq := BindReq{
			Key:    key,
			NodeID: cfg.AgentID,
			Sign:   hex.EncodeToString(sign),
		}

		buf, err := json.Marshal(bindReq)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to marshal bind request: %s", err.Error()), -1)
		}

		ctx, cancel := context.WithTimeout(cctx.Context, 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", webUrl, bytes.NewReader(buf))
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to create request: %s", err.Error()), -1)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{
			Timeout:   10 * time.Second,
			Transport: ahttp.DefaultDNSRountTripper,
		}

		resp, err := client.Do(req)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to bind: %s", err.Error()), -1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			buf, err := io.ReadAll(resp.Body)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to read response body: %s", err.Error()), -1)
			}
			return cli.Exit(fmt.Errorf("bind failed, status code %d, response body: %s", resp.StatusCode, string(buf)), -1)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to read response body: %s", err.Error()), -1)
		}

		type Resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		rsp := Resp{}
		if err := json.Unmarshal(respBody, &rsp); err != nil {
			return cli.Exit(fmt.Errorf("failed to unmarshal response body: %s", err.Error()), -1)
		}

		if rsp.Code != 0 {
			return cli.Exit(fmt.Errorf("bind failed, code: %d, msg: %s", rsp.Code, rsp.Msg), -1)
		}
		return nil
	},
}

// var (
// 	currentLogFile *os.File
// 	logMu          sync.Mutex
// )

// func setupDailyRotatingLog(workingDir, baseName string, keepDays int) {
// 	if err := os.MkdirAll(workingDir, 0755); err != nil {
// 		log.Fatalf("Failed to create log directory: %v", err)
// 	}

// 	rotateLogFile(workingDir, baseName)

// 	go func() {
// 		cleanOldLogs(workingDir, baseName, keepDays)
// 		ticker := time.NewTicker(24 * time.Hour)
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			cleanOldLogs(workingDir, baseName, keepDays)
// 		}
// 	}()

// 	go func() {
// 		for {
// 			now := time.Now()
// 			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 5, 0, now.Location())
// 			time.Sleep(time.Until(nextMidnight))

// 			logMu.Lock()
// 			rotateLogFile(workingDir, baseName)
// 			logMu.Unlock()
// 		}
// 	}()
// }

// func rotateLogFile(workingDir, baseName string) {
// 	if currentLogFile != nil {
// 		currentLogFile.Close()
// 	}

// 	currentDate := time.Now().Format("2006-01-02")
// 	logFileName := fmt.Sprintf("%s-%s.log", strings.TrimSuffix(baseName, ".log"), currentDate)
// 	fullPath := path.Join(workingDir, logFileName)

// 	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
// 	if err != nil {
// 		log.Printf("Failed to create log file: %v", err)
// 		return
// 	}

// 	currentLogFile = file
// 	os.Stdout = file
// 	os.Stderr = file

// 	log.Printf("========== Rotated to new log file: %s ==========", logFileName)
// }

// // baseName agent.log
// func cleanOldLogs(dir, baseName string, keepDays int) {
// 	if keepDays <= 0 {
// 		return
// 	}
// 	files, err := os.ReadDir(dir)
// 	if err != nil {
// 		log.Printf("Failed to read log directory: %v", err)
// 		return
// 	}
// 	prefix := strings.TrimSuffix(baseName, ".log") + "-"
// 	cutoff := time.Now().AddDate(0, 0, -keepDays).Unix()
// 	for _, file := range files {
// 		if file.IsDir() {
// 			continue
// 		}

// 		filename := file.Name()
// 		if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, ".log") {
// 			continue
// 		}

// 		dateStr := strings.TrimPrefix(filename, prefix)
// 		dateStr = strings.TrimSuffix(dateStr, ".log")

// 		logDate, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
// 		if err != nil {
// 			log.Error(err)
// 			continue
// 		}

// 		if logDate.Unix() < cutoff {
// 			fullPath := filepath.Join(dir, filename)
// 			if err := os.Remove(fullPath); err != nil {
// 				log.Printf("Failed to remove old log %s: %v", filename, err)
// 			} else {
// 				log.Printf("Removed old log file: %s", filename)
// 			}
// 		}
// 	}
// }

func main() {
	commands := []*cli.Command{
		runCmd,
		versionCmd,
		testCmd,
		bindCmd,
	}

	app := &cli.App{
		Name:     "controller",
		Usage:    "Manager and update application",
		Commands: commands,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

const (
	logoWindow = `
╭━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╮
┃   ▀█▀   █   ▀█▀   ▄▀█   █▄░█         ┃
┃   ░█░   █   ░█░   █▀█   █░▀█         ┃
┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┃
┃           4th Galileo TestNet        ┃
┃               Version 0.1.3          ┃
╰━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╯
`

//	MonitorWindow = `
//
// ╔════════════════════ TITAN NETWORK ════════════════════╗
// ║                                                       ║
// ║  [STATUS: RUNNING] >>> Node Controller Active <<<     ║
// ║                                                       ║
// ║  Dashboard: https://www.test-api.titannet.io          ║
// ║  Agent ID : titan43ac28b7-b902-4597-8b22-b670e9712a0d ║
// ║                                                       ║
// ╚═══════════════════════════════════════════════════════╝
// `
)
