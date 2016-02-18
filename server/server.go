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

const POM_TIME time.Duration = time.Second * 5
const OSX_CMD string = "osascript"

const SUCCESS string = "success"
const CANCELLED string = "cancelled"
const PAUSED string = "paused"

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
	eventBus   chan PomEvent

	pomStore      PomStore
	currentPom    *Pom
	currentTimer  *time.Timer
	lastStartTime time.Time
}

func (app *App) apiClearDB(res http.ResponseWriter, req *http.Request) {
	err := app.pomStore.Clear()
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Failed to clear database"))
	} else {
		res.WriteHeader(http.StatusNoContent)
		res.Write([]byte("Pom store cleared"))
	}
}

func (app *App) apiStopHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		fmt.Println("Stopping Pom", app.currentPom.name)
		// TODO: Should wrap these in helper functions
		app.runningPom = false
		app.pomStore.StoreStatus(CANCELLED, app.currentPom)
		app.currentPom.timer.Stop()
		app.currentPom = nil
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been cancelled"))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("No current pom to cancel"))
	}
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if !app.runningPom { // TODO: add has pom method with helper functions
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
			app.pomStore.StoreStatus(SUCCESS, app.currentPom)
			app.eventBus <- PomEvent{eventType: "Success", title: "Oswald", message: "Pom Finished"}
			app.currentTimer = nil
		}()
		res.WriteHeader(http.StatusCreated)
		// TODO: wrap up in time left func
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
		// TODO: Better output handling
		res.Write([]byte(fmt.Sprintf("Currently in pom %s, ~%d minutes left", app.currentPom.name, int(mintuesLeft.Minutes()))))
	} else {
		// TODO: Use pomStore
		successCount, err := app.pomStore.GetStatusCount(SUCCESS) // TODO: Wrap in errGet interface?
		if err != nil {
			fmt.Println("Error getting status count", err)
		}
		cancelledCount, err := app.pomStore.GetStatusCount(CANCELLED) // TODO: Wrap in errGet interface?
		if err != nil {
			fmt.Println("Error getting status count", err)
		}
		pausedCount, err := app.pomStore.GetStatusCount(PAUSED) // TODO: Wrap in errGet interface?
		if err != nil {
			fmt.Println("Error getting status count", err)
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(fmt.Sprintf("Success: %d, Cancelled: %d, Paused: %d", successCount, cancelledCount, pausedCount)))
	}
}

// TODO: Add interface for this
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

	// TODO: Should the eventbus be more robust?
	notifications := make(chan PomEvent)
	pomStore := NewBoltPomStore()

	app := &App{
		runningPom: false,
		eventBus:   notifications,
		pomStore:   pomStore,
	}
	// TODO: move into app, create a 'start' or 'run' method
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
	r.HandleFunc("/clear", app.apiClearDB)

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
