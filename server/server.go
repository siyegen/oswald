package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/mux"
)

const POM_TIME time.Duration = time.Second * 15
const OSX_CMD string = "osascript"

type App struct {
	runningPom bool
	results    map[string]int
	notifier   *OSXNotifier
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if !app.runningPom {
		fmt.Println("Starting Pom")
		app.runningPom = true
		pomTimer := time.NewTimer(POM_TIME)
		go func() {
			<-pomTimer.C
			fmt.Println("Finished POM")
			app.results["Success"] = app.results["Success"] + 1
			app.runningPom = false
			err := app.notifier.sendNotification("Pom finished", "Oswald")
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
}

func (app *App) apiStatusHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		res.WriteHeader(http.StatusConflict)
		res.Write([]byte("Currently in pom, %time left"))
	} else {
		success := app.results["Success"]
		cancelled := app.results["Cancelled"]
		paused := app.results["Paused"]
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(fmt.Sprintf("Success: %d, Cancelled: %d, Paused: %d", success, cancelled, paused)))
	}
}

type OSXNotifier struct {
	baseCommand string
	flag        string
}

func (n *OSXNotifier) sendNotification(message, title string) error {
	fullMessage := fmt.Sprintf("display notification \"%s\" with title \"%s\" sound name \"Submarine\"", message, title)
	cmdArgs := []string{n.flag, fullMessage}
	_, err := exec.Command(n.baseCommand, cmdArgs...).Output()
	return err
}

func main() {
	osxNotifier := &OSXNotifier{OSX_CMD, "-e"}

	app := &App{
		runningPom: false,
		notifier:   osxNotifier,
		results:    map[string]int{"Success": 0, "Cancelled": 0, "Paused": 0},
	}

	portString := ":13381"
	r := mux.NewRouter()

	// api endpoints
	r.HandleFunc("/start", app.apiStartHandler)
	r.HandleFunc("/status", app.apiStatusHandler)

	r.HandleFunc("/stop", func(res http.ResponseWriter, req *http.Request) {
		fmt.Println("stop Pom")
	})

	fmt.Println("Starting Server at:", portString)
	http.ListenAndServe(portString, r)
	fmt.Println("Shutting down")
}
