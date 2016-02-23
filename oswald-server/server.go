package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const LOG_PREFIX = "[Oswald]"

const POM_TIME time.Duration = time.Second * 5
const OSX_CMD string = "osascript"

var logger *log.Logger = log.New(os.Stdout, LOG_PREFIX, log.LstdFlags)

type StatusString string

const SUCCESS StatusString = "success"
const CANCELLED StatusString = "cancelled"
const PAUSED StatusString = "paused"

type PomEvent struct {
	eventType string
	title     string
	message   string
}

type Pom struct {
	startTime time.Time
	name      string
	timer     *time.Timer
	running   bool

	done chan bool
	stop chan struct{}
}

func NewPom(optionalName string) *Pom {
	return &Pom{
		name: optionalName,
		done: make(chan bool),
		stop: make(chan struct{}),
	}
}

func (p *Pom) FinishTime() time.Time {
	return p.startTime.Add(POM_TIME)
}

func (p *Pom) Start() {
	p.running = true
	p.timer = time.NewTimer(POM_TIME)
	p.startTime = time.Now()

	logger.Println("Starting Pom", p.name)
	go func() {
		select {
		case <-p.timer.C:
			logger.Println("Finished Pom")
			p.running = false
			p.done <- true
		case <-p.stop:
			logger.Println("Stopped Pom")
			p.running = false
			p.done <- false
		}
	}()
}

func (p *Pom) Stop() {
	p.running = false
	p.timer.Stop()

	logger.Println("Stopping Pom", p.name)
	p.stop <- struct{}{}
}

type App struct {
	runningPom bool
	eventBus   chan PomEvent

	pomStore      PomStore
	currentPom    *Pom
	lastStartTime time.Time
}

func (app *App) handleTimer() {
	go func() {
		success := <-app.currentPom.done
		app.runningPom = false
		if success {
			logger.Println("Finished Pom, from startPom method", app.currentPom.name)
			app.pomStore.StoreStatus(SUCCESS, *app.currentPom)
			app.eventBus <- PomEvent{eventType: "Success", title: "Oswald", message: "Pom Finished"}
		} else {
			logger.Println("Cancelled Pom, from startPom method", app.currentPom.name)
			app.pomStore.StoreStatus(CANCELLED, *app.currentPom)
			app.eventBus <- PomEvent{eventType: "Stopped", title: "Oswald", message: "Pom Cancelled"}
		}
		// app.currentPom = nil
	}()
}

func (app *App) startPom(optName string) {
	app.currentPom = NewPom(optName)
	app.runningPom = true

	app.currentPom.Start()
	app.handleTimer()
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if !app.runningPom { // TODO: add inPom method with helper functions
		vars := mux.Vars(req)
		optName, _ := vars["name"]

		app.startPom(optName)
		startTime := app.currentPom.startTime.Format(time.Kitchen)
		finishTime := app.currentPom.FinishTime().Format(time.Kitchen)
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Pom already running, pause or cancel first"))
	}
}

func (app *App) apiStopHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		app.currentPom.Stop()
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been cancelled"))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("No current pom to cancel"))
	}
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

func (app *App) apiStatusHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		mintuesLeft := (app.currentPom.startTime.Add(POM_TIME)).Sub(time.Now())
		res.WriteHeader(http.StatusConflict)
		// TODO: Better output handling
		res.Write([]byte(fmt.Sprintf("Currently in pom %s, ~%d minutes left", app.currentPom.name, int(mintuesLeft.Minutes()))))
	} else {
		successCount, err := app.pomStore.GetStatusCount(SUCCESS) // TODO: Wrap in errGet interface?
		if err != nil {
			logger.Printf("Error getting status count %s", err)
		}
		cancelledCount, err := app.pomStore.GetStatusCount(CANCELLED) // TODO: Wrap in errGet interface?
		if err != nil {
			logger.Printf("Error getting status count %s", err)
		}
		pausedCount, err := app.pomStore.GetStatusCount(PAUSED) // TODO: Wrap in errGet interface?
		if err != nil {
			logger.Printf("Error getting status count %s", err)
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(fmt.Sprintf("Success: %d, Cancelled: %d, Paused: %d", successCount, cancelledCount, pausedCount)))
	}
}

type OSNotifier interface {
	SendNotification(message, title string) error
}

type CmdLineNotifier struct{}

func (n *CmdLineNotifier) SendNotification(message, title string) error {
	fmt.Printf("[%s]: %s\n", title, message)
	return nil
}

type OSXNotifier struct {
	baseCommand string
	flag        string
}

func (n *OSXNotifier) SendNotification(message, title string) error {
	fullMessage := fmt.Sprintf("display notification \"%s\" with title \"%s\" sound name \"Submarine\"", message, title)
	cmdArgs := []string{n.flag, fullMessage}
	_, err := exec.Command(n.baseCommand, cmdArgs...).Output()
	return err
}

func main() {
	logger.Println("Starting Up")
	var notifier OSNotifier
	if runtime.GOOS == "darwin" {
		notifier = &OSXNotifier{OSX_CMD, "-e"}
	} else {
		notifier = &CmdLineNotifier{}
	}

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
			notifier.SendNotification(event.message, event.title)
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
		logger.Fatalf("Error creating listener %s", err.Error())
	}

	go http.Serve(listener, r)
	logger.Printf("Listening at %s", portString)

	<-done
	logger.Println("Shutting down")

}
