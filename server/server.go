package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const POM_TIME time.Duration = time.Minute * 25
const OSX_CMD string = "osascript"

type App struct {
	runningPom bool
	results    map[string]int
	eventBus   chan PomEvent

	currentTimer  *time.Timer
	lastStartTime time.Time
}

type PomEvent struct {
	eventType string
	title     string
	message   string
}

func (app *App) apiStopHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		fmt.Println("Stopping Pom")
		// wrap in some helper functions?
		app.runningPom = false
		app.currentTimer.Stop()
		app.results["Cancelled"] = app.results["Cancelled"] + 1
		app.currentTimer = nil
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been cancelled"))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("No current pom to cancel"))
	}
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if !app.runningPom {
		vars := mux.Vars(req)
		optName, _ := vars["name"]
		fmt.Println("Starting Pom", optName)
		app.runningPom = true
		app.currentTimer = time.NewTimer(POM_TIME)
		app.lastStartTime = time.Now()
		go func() {
			<-app.currentTimer.C
			app.runningPom = false
			fmt.Println("Finished POM")
			// wrapper for these? Moving to boltdb anyway so might as well wait
			app.results["Success"] = app.results["Success"] + 1
			app.eventBus <- PomEvent{eventType: "Success", title: "Oswald", message: "Pom Finished"}
			app.currentTimer = nil
		}()
		res.WriteHeader(http.StatusCreated)
		startTime := app.lastStartTime.Format(time.Kitchen)
		finishTime := app.lastStartTime.Add(time.Minute * 25).Format(time.Kitchen)
		res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Pom already running, pause or cancel first"))
	}
}

func (app *App) apiStatusHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		mintuesLeft := (app.lastStartTime.Add(POM_TIME)).Sub(time.Now())
		res.WriteHeader(http.StatusConflict)
		res.Write([]byte(fmt.Sprintf("Currently in pom, ~%d minutes left", int(mintuesLeft.Minutes()))))
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

	sigs := make(chan os.Signal)
	done := make(chan struct{})

	// eventbus for notifications
	notifications := make(chan PomEvent)

	app := &App{
		runningPom: false,
		eventBus:   notifications,
		results:    map[string]int{"Success": 0, "Cancelled": 0, "Paused": 0},
	}
	go func(eventChannel chan PomEvent) {
		for {
			event := <-eventChannel
			osxNotifier.sendNotification(event.message, event.title)
		}
	}(notifications)

	portString := ":13381"
	r := mux.NewRouter()

	// api endpoints
	r.HandleFunc("/start/{name}", app.apiStartHandler)
	r.HandleFunc("/status", app.apiStatusHandler)
	r.HandleFunc("/stop", app.apiStopHandler)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- struct{}{}
	}()
	listener, err := net.Listen("tcp", portString)
	if err != nil {
		fmt.Errorf("Error creating listener", err)
	}

	fmt.Println("Starting Server at:", portString)
	go http.Serve(listener, r)

	<-done
	fmt.Println("\nShutting down")

}
