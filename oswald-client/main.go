package main

import (
	"fmt"
	"os/exec"
	"time"
)

const POM_TIME time.Duration = time.Minute * 25

const OSX_CMD string = "osascript"

type OSXNotifier struct {
	baseCommand string
	flag        string
	// messageArg  string
}

func (n *OSXNotifier) sendNotification(message, title string) error {
	fullMessage := fmt.Sprintf("display notification \"%s\" with title \"%s\"", message, title)
	cmdArgs := []string{n.flag, fullMessage}
	_, err := exec.Command(OSX_CMD, cmdArgs...).Output()
	return err
}

func main() {
	fmt.Println("Starting a pom!")
	nnn := &OSXNotifier{OSX_CMD, "-e"}
	// pomTimer := time.NewTimer(POM_TIME)
	// <-pomTimer.C
	// out, err := exec.Command(cmd, argsAA...).Output()
	err := nnn.sendNotification("I'm mister message", "Title bitch")
	if err != nil {
		fmt.Println("Error", err)
	}

	fmt.Println("Finished a pom!")
}
