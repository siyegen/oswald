package main

import (
	"fmt"
	"os/exec"
	"time"
)

const POM_TIME time.Duration = time.Minute * 25
const cmd string = "osascript"

var argsAA = []string{"-e", "display notification \"moo\" with title \"MyTitle\""}

func main() {
	fmt.Println("Starting a pom!")
	// pomTimer := time.NewTimer(POM_TIME)
	// <-pomTimer.C
	out, err := exec.Command(cmd, argsAA...).Output()
	if err != nil {
		fmt.Println("Error", err)
	} else {
		fmt.Println("Output", string(out))
	}
	fmt.Println("Finished a pom!")
}
