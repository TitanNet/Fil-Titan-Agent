package main

import (
	"agent/agent"
	"agent/common"
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "agent",
		Usage: "Manager and update business process",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "working-dir",
				Usage:    "--working-dir=/path/to/working/dir",
				EnvVars:  []string{"WORKING_DIR"},
				Required: true,
				Value:    "",
			},
			&cli.StringFlag{
				Name:    "script-file-name",
				Usage:   "--script-file-name script.lua",
				EnvVars: []string{"SCRIPT_FILE_NAME"},
				Value:   "script.lua",
			},

			&cli.IntFlag{
				Name:    "script-interval",
				Usage:   "--script-interval 60",
				EnvVars: []string{"SCRIPT_INTERVAL"},
				Value:   60,
			},
			&cli.StringFlag{
				Name:     "server-url",
				Usage:    "--server-url http://localhost:8080/update/lua",
				EnvVars:  []string{"SERVER_URL"},
				Required: true,
				Value:    "http://localhost:8080/update/lua",
			},
			&cli.StringFlag{
				Name:  "channel",
				Usage: "--channel titan-l1, channel: titan-l1,painet,emc-titan-l2",
				Value: "",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "--key YOUR_WEB_KEY",
				Value: "",
			},
			&cli.StringFlag{
				Name:    "log-path",
				Usage:   "--log-path /var/log",
				EnvVars: []string{"AGENT_LOG_PATH"},
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "log-file",
				Usage:   "--log-file agent.log",
				EnvVars: []string{"AGENT_LOG_FILE"},
				Value:   "agent.log",
			},
			&cli.IntFlag{
				Name:    "log-keep-days",
				Usage:   "--log-keep-days 3",
				EnvVars: []string{"LOG_KEEP_DAYS"},
				Value:   10,
			},

			&cli.BoolFlag{
				Name:  "autostart",
				Usage: "--autostart automatically start the agent on system startup",
				Value: false,
			},
		},
		Before: func(cctx *cli.Context) error {

			logFile := cctx.String("log-file")
			keepDays := cctx.Int("log-keep-days")
			logPath := cctx.String("log-path")
			if logPath == "" {
				logPath = cctx.String("working-dir")
			}
			if logPath == "" {
				log.Fatalf("log path is required")
			}
			if logFile != "" {
				rOut, wOut, err := os.Pipe()
				if err != nil {
					log.Fatal(err)
				}
				rErr, wErr, err := os.Pipe()
				if err != nil {
					log.Fatal(err)
				}

				os.Stdout = wOut
				os.Stderr = wErr

				logflusher := common.NewLogRotator(cctx.Context, logPath, logFile, 24*time.Hour, keepDays, rOut, rErr)
				log.SetOutput(os.Stdout)
				go logflusher.Start()
			}
			return nil
		},
		Action: func(cctx *cli.Context) error {
			workingDir := cctx.String("working-dir")
			if workingDir == "" {
				log.Fatalf("working-dir is required")
			}

			err := os.MkdirAll(workingDir, os.ModePerm)
			if err != nil {
				log.Fatalf("create working-dir failed:%s", err.Error())
			}

			args := &agent.AgentArguments{
				WorkingDir:     cctx.String("working-dir"),
				ScriptFileName: cctx.String("script-file-name"),

				ScriptInvterval: cctx.Int("script-interval"),
				ServerURL:       cctx.String("server-url"),
				Channel:         cctx.String("channel"),
				Key:             cctx.String("key"),
				AutoStart:       cctx.Bool("autostart"),
			}

			agent, err := agent.New(args)
			if err != nil {
				log.Fatal(err)
			}

			if args.AutoStart {
				return agent.AutoStart()
			}

			ctx, done := context.WithCancel(cctx.Context)
			sigChan := make(chan os.Signal, 2)
			go func() {
				<-sigChan
				done()
			}()

			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
			return agent.Run(ctx)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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
