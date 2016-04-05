package main

import (
	"flag"
	"fmt"
)

func main() {
	start := flag.String("start", "", "pom to start")
	cancel := flag.Bool("cancel", false, "pom to cancel")
	pause := flag.Bool("pause", false, "pom to pause")
	resume := flag.Bool("resume", false, "pom to resume")
	status := flag.Bool("status", false, "see pom status")
	clear := flag.Bool("clear", false, "pom to clear")
	flag.Parse()

	client := New()

	fmt.Println(*status)

	switch {
	case *start != "":
		client.Start(*start)
	case *cancel:
		client.Cancel()
	case *pause:
		client.Pause()
	case *resume:
		client.Resume()
	case *status:
		client.Status()
	case *clear:
		client.Clear()
	default:
		flag.Usage()
	}
}
