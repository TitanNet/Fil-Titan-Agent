package agent

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os"
	"strings"
)

func (a *Agent) AutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	cmdN := fmt.Sprintf("%s %s", exePath, strings.Join(os.Args[1:], " "))

	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	a.Run(context.Background())

	return key.SetStringValue("TitanAgent", cmdN)
}
