package main

import (
	"encoding/json"
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

const LOG_PREFIX string = "[Oswald]"
const POM_TIME time.Duration = time.Minute * 1

const OSX_CMD string = "osascript"

var logger *log.Logger = log.New(os.Stdout, LOG_PREFIX, log.LstdFlags)

type StatusString string

const SUCCESS StatusString = "success"
const CANCELLED StatusString = "cancelled"
const PAUSED StatusString = "paused"

type PomEvent struct {
	name      string
	eventType string
	title     string
	message   string
	time      time.Time
}

func NewPomEvent(eventType, title, message, name string, time time.Time) PomEvent {
	return PomEvent{
		name:      name,
		eventType: eventType,
		title:     title,
		message:   message,
		time:      time,
	}
}

type App struct {
	eventBus chan PomEvent

	pomStore      PomStore
	currentPom    *Pom
	lastStartTime time.Time
}

func (app *App) startTimerHandler() {
	go func() {
		success := <-app.currentPom.done
		if success {
			logger.Println("Finished Pom, from startPom method", app.currentPom.name)
			event := NewPomEvent("Success", "Oswald", "Pom Finished", app.currentPom.name, time.Now())
			app.pomStore.StoreStatus(SUCCESS, event)
			app.eventBus <- event
		} else {
			logger.Println("Cancelled Pom, from startPom method", app.currentPom.name)
			event := NewPomEvent("Success", "Oswald", "Pom Finished", app.currentPom.name, time.Now())
			app.pomStore.StoreStatus(CANCELLED, event)
			app.eventBus <- event
		}
	}()
}

func jsonStart() {
	// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	// state
	// start time
	// finish time
	// pause time -na
	// message
	// name
}

func (app *App) apiStartHandler(res http.ResponseWriter, req *http.Request) {
	if app.currentPom.State() == None {
		vars := mux.Vars(req)
		optName, _ := vars["name"]

		app.currentPom = NewPom(optName, POM_TIME)
		app.currentPom.Start()
		app.startTimerHandler()

		// TODO: Cleanup into formatter
		// startTime := app.currentPom.startTime.Format(time.Kitchen)
		// finishTime := app.currentPom.FinishTime().Format(time.Kitchen)
		res.WriteHeader(http.StatusCreated)
		json.NewEncoder(res).Encode(app.currentPom.ToPomMessage())
		// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Pom already running, pause or cancel first"))
	}
}

func jsonCancel() {
	// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	// state
	// start time
	// finish time -na
	// timeSpentPaused
	// message
	// name
}

func (app *App) apiCancelHandler(res http.ResponseWriter, req *http.Request) {
	if app.currentPom.State() == Running || app.currentPom.State() == Paused {
		app.currentPom.Stop()
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been cancelled"))
	} else {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("No current pom to cancel"))
	}
}

func jsonPause() {
	// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	// state
	// start time
	// finish time
	// pause time
	// message
	// name
}

func (app *App) apiPauseHandler(res http.ResponseWriter, req *http.Request) {
	if app.currentPom.State() == Running {
		app.currentPom.Pause()
		event := NewPomEvent("Paused", "Oswald", "Pom Paused", app.currentPom.name, time.Now())
		app.pomStore.StoreStatus(PAUSED, event)
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Pom has been paused"))
	} else {
		res.WriteHeader(http.StatusConflict)
		res.Write([]byte("No pom to pause"))
	}
}

func jsonResume() {
	// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	// state
	// start time
	// finish time
	// timeSpentPaused
	// message
	// name
}

func (app *App) apiResumeHandler(res http.ResponseWriter, req *http.Request) {
	if app.currentPom.State() == Paused {
		app.currentPom.Resume()
		res.WriteHeader(http.StatusAccepted)
		res.Write([]byte("Resuming pom"))
	} else {
		res.WriteHeader(http.StatusConflict)
		res.Write([]byte("No pom to pause"))
	}
}

// special case json
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

func jsonStatus() {
	// res.Write([]byte(fmt.Sprintf("Started POM at %s, will end at %s", startTime, finishTime)))
	// state
	// start time
	// finish time
	// timeSpentPaused
	// message
	// name
}

func (app *App) apiStatusHandler(res http.ResponseWriter, req *http.Request) {
	if app.currentPom.State() == Running {
		res.WriteHeader(http.StatusConflict)
		// TODO: Better output handling
		minutesLeft := app.currentPom.TimeLeft().Minutes()
		res.Write([]byte(fmt.Sprintf("Currently in pom %s, ~%d minutes left", app.currentPom.name, int(minutesLeft))))
	} else if app.currentPom.State() == Paused {
		res.WriteHeader(http.StatusConflict)
		// TODO: Better output handling
		res.Write([]byte(fmt.Sprintf("Pom currently paused, %s left", app.currentPom.TimeLeft())))
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

	notifications := make(chan PomEvent)
	pomStore := NewBoltPomStore()

	app := &App{
		eventBus: notifications,
		pomStore: pomStore,
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
	r.HandleFunc("/cancel", app.apiCancelHandler)
	r.HandleFunc("/pause", app.apiPauseHandler)
	r.HandleFunc("/resume", app.apiResumeHandler)
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
