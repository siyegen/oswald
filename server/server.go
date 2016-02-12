package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/mux"
)

const POM_TIME time.Duration = time.Minute * 25
const OSX_CMD string = "osascript"

func main() {
	cmdArgs := []string{"-e", "display notification \"Finished POM!\" with title \"Oswald:\""}
	results := map[string]int{"Success": 0, "Cancelled": 0, "Paused": 0}
	portString := ":13380"
	runningPom := false
	fmt.Println("Starting Server at:", portString)
	// Start a server
	r := mux.NewRouter()

	// api endpoints
	r.HandleFunc("/start", func(res http.ResponseWriter, req *http.Request) {
		if !runningPom {
			fmt.Println("Starting Pom")
			runningPom = true
			pomTimer := time.NewTimer(POM_TIME)
			go func() {
				<-pomTimer.C
				fmt.Println("Finished POM")
				results["Success"] = results["Success"] + 1
				runningPom = false
				_, err := exec.Command(OSX_CMD, cmdArgs...).Output()
				if err != nil {
					fmt.Println("Failed to send notifications", err)
				}
			}()
			res.WriteHeader(http.StatusCreated)
			res.Write([]byte(fmt.Sprintf("Started POM at %s", time.Now())))
		} else {
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte("Pom already running, pause or cancel first"))
		}
	})

	r.HandleFunc("/stop", func(res http.ResponseWriter, req *http.Request) {
		fmt.Println("stop Pom")
	})

	r.HandleFunc("/status", func(res http.ResponseWriter, req *http.Request) {
		if runningPom {
			res.WriteHeader(http.StatusConflict)
			res.Write([]byte("Currently in pom, %time left"))
		} else {
			success := results["Success"]
			cancelled := results["Cancelled"]
			paused := results["Paused"]
			res.WriteHeader(http.StatusOK)
			res.Write([]byte(fmt.Sprintf("Success: %d, Cancelled: %d, Paused: %d", success, cancelled, paused)))
		}
	})

	http.ListenAndServe(portString, r)
	fmt.Println("Shutting down")
}
