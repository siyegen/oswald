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

const POM_TIME time.Duration = time.Second * 5
const OSX_CMD string = "osascript"

const UID_BUCKET string = "__oswald_uid"
const SUCCESS string = "success"
const CANCELLED string = "cancelled"
const PAUSED string = "paused"

type PomStore interface {
	StoreStatus(status string, pom *Pom) error
	// GetStatus(status string) error
	GetStatusCount(status string) (int, error)
}

type BoltPomStore struct {
	uid    []byte
	db     *bolt.DB
	dbName string
}

func NewBoltPomStore() PomStore {
	name := "_dev.db"
	db, err := bolt.Open(fmt.Sprintf("dev_db/%s", name), 0600, nil)
	if err != nil {
		fmt.Errorf("Error opening db %s", err)
	}
	var uid []byte
	uidKey := []byte("uid")
	// TODO: See if we can clean this up or move out
	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(UID_BUCKET))
		if err != nil {
			return err
		}
		if existingUid := bucket.Get(uidKey); existingUid != nil {
			uid = existingUid
		} else {
			uid = []byte(newUUID())
			bucket.Put(uidKey, uid)
		}
		return nil
	})
	if err != nil {
		fmt.Errorf("Error opening db %s", err)
	}
	fmt.Println("our uuid", string(uid))
	return &BoltPomStore{db: db, dbName: name, uid: uid}
}

// TODO: Implement these
func (b *BoltPomStore) StoreStatus(status string, pom *Pom) error { // REVIEW: Should pom be pomEvent?
	return b.db.Update(func(tx *bolt.Tx) error {
		fmt.Println("using uuid", string(b.uid), "for", status)
		bucket, err := tx.CreateBucketIfNotExists(b.uid) // TODO: Really, clean this up...
		if err != nil {
			fmt.Println("Couldn't create bucket", status)
			return err
		}
		statusBucket, err := bucket.CreateBucketIfNotExists([]byte(status))
		if err != nil {
			fmt.Println("Error with bucket storeStatus", status)
			return err
		}
		nextId, _ := statusBucket.NextSequence()
		sortableTime := []byte(pom.startTime.Format(time.RFC3339))
		fmt.Println("Storing...", nextId)
		return statusBucket.Put(sortableTime, itob(int(nextId)))
	})
}

func (b *BoltPomStore) GetStatusCount(status string) (int, error) {
	count := 0
	b.db.View(func(tx *bolt.Tx) error {
		fmt.Println("using uuid", string(b.uid), "for", status)
		bucket := tx.Bucket(b.uid)
		fmt.Println("Bucket", bucket, "status", status)
		statusBucket := bucket.Bucket([]byte(status))
		if statusBucket == nil { // Assume no count
			fmt.Println("statusBucket", statusBucket)
			return nil
		}
		key, value := statusBucket.Cursor().Last()
		fmt.Println("key, value", string(key), value)
		count = btoi(value)
		return nil
	})
	return count, nil
}

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
	results    map[string]int // TODO: Remove once pomStore works
	eventBus   chan PomEvent

	pomStore      PomStore
	currentPom    *Pom
	currentTimer  *time.Timer
	lastStartTime time.Time
}

func (app *App) apiStopHandler(res http.ResponseWriter, req *http.Request) {
	if app.runningPom {
		fmt.Println("Stopping Pom", app.currentPom.name)
		// TODO: Should wrap these in helper functions
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
			app.results["Success"] = app.results["Success"] + 1
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
		success := app.results["Success"]
		cancelled := app.results["Cancelled"]
		paused := app.results["Paused"]
		count, _ := app.pomStore.GetStatusCount(SUCCESS)
		fmt.Println("Success status:", count)
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(fmt.Sprintf("Success: %d, Cancelled: %d, Paused: %d", success, cancelled, paused)))
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
		results:    map[string]int{"Success": 0, "Cancelled": 0, "Paused": 0},
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
