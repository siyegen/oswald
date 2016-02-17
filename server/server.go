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

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)

const POM_TIME time.Duration = time.Minute * 25
const OSX_CMD string = "osascript"

type PomEvent struct {
	eventType string
	title     string
	message   string
}

type Pom struct {
	startTime time.Time
	name      string
	timer     *time.Timer
}

func NewPom(optionalName string) *Pom {
	return &Pom{name: optionalName}
}

type App struct {
	runningPom bool
	results    map[string]int
	eventBus   chan PomEvent

	currentPom    *Pom
	currentTimer  *time.Timer
	lastStartTime time.Time
}

func (app *App) apiStopHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		fmt.Println("Stopping Pom", app.currentPom.name)
		// wrap in some helper functions?
		app.runningPom = false
		app.currentPom.timer.Stop()
		app.currentPom = nil
		app.results["Cancelled"] = app.results["Cancelled"] + 1
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been cancelled"))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("No current pom to cancel"))
	}
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if !app.runningPom { // has pom method?
		vars := mux.Vars(req)
		optName, _ := vars["name"]

		pom := NewPom(optName)
		app.currentPom = pom
		app.runningPom = true
		app.currentPom.timer = time.NewTimer(POM_TIME)
		app.currentPom.startTime = time.Now()

		fmt.Println("Starting Pom", app.currentPom.name)
		go func() {
			<-app.currentPom.timer.C
			app.runningPom = false
			fmt.Println("Finished POM", app.currentPom.name)
			// wrapper for these? Moving to boltdb anyway so might as well wait
			app.results["Success"] = app.results["Success"] + 1
			app.eventBus <- PomEvent{eventType: "Success", title: "Oswald", message: "Pom Finished"}
			app.currentTimer = nil
		}()
		res.WriteHeader(http.StatusCreated)
		// wrap up
		startTime := app.currentPom.startTime.Format(time.Kitchen)
		finishTime := app.currentPom.startTime.Add(time.Minute * 25).Format(time.Kitchen)
		res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Pom already running, pause or cancel first"))
	}
}

func (app *App) apiStatusHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		mintuesLeft := (app.currentPom.startTime.Add(POM_TIME)).Sub(time.Now())
		res.WriteHeader(http.StatusConflict)
		// handle optional name better
		res.Write([]byte(fmt.Sprintf("Currently in pom %s, ~%d minutes left", app.currentPom.name, int(mintuesLeft.Minutes()))))
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
	db, err := bolt.Open("dev_db/_dev.db", 0600, nil)
	if err != nil {
		fmt.Errorf("Error opening db %s", err)
	}
	fmt.Println("DB", db)

	app := &App{
		runningPom: false,
		eventBus:   notifications,
		results:    map[string]int{"Success": 0, "Cancelled": 0, "Paused": 0},
	}
	// move into app, create a 'start' or 'run' method
	go func(eventChannel chan PomEvent) {
		for {
			event := <-eventChannel
			osxNotifier.sendNotification(event.message, event.title)
		}
	}(notifications)

	portString := ":13381"
	r := mux.NewRouter()

	// api endpoints
	r.HandleFunc("/start", app.apiStartHandler)
	r.HandleFunc("/start/{name}", app.apiStartHandler)
	r.HandleFunc("/status", app.apiStatusHandler)
	r.HandleFunc("/stop", app.apiStopHandler)

	// better way to handle this?
	// also client should send SIGINT to shutdown
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- struct{}{}
	}()
	// move into app start / settings?
	listener, err := net.Listen("tcp", portString)
	if err != nil {
		fmt.Errorf("Error creating listener", err)
	}

	fmt.Println("Starting Server at:", portString)
	go http.Serve(listener, r)

	<-done
	fmt.Println("\nShutting down")

}
