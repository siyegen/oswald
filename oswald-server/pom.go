package main

import (
	"encoding/json"
	"strconv"
	"time"
)

type PomState int

const (
	None PomState = iota
	Running
	Paused
)

func (ps PomState) String() string {
	if ps == None {
		return "None"
	} else if ps == Running {
		return "Running"
	} else if ps == Paused {
		return "Paused"
	}
	return ""
}

var stateMessageLookup = map[string]map[PomState]map[PomState]string{
	"start":  map[PomState]map[PomState]string{None: map[PomState]string{Running: "Started Pom"}, Running: map[PomState]string{Running: "Currently in Pom"}, Paused: map[PomState]string{Paused: "Pom is paused, Resume to continue"}},
	"cancel": map[PomState]map[PomState]string{None: map[PomState]string{None: "No Pom to cancel"}, Running: map[PomState]string{None: "Cancelled Pom"}, Paused: map[PomState]string{None: "Cancelled Pom"}},
	"pause":  map[PomState]map[PomState]string{Running: map[PomState]string{Paused: "Paused Pom"}, Paused: map[PomState]string{Paused: "Pom is already paused"}, None: map[PomState]string{None: "No Pom to pause"}},
	"resume": map[PomState]map[PomState]string{Paused: map[PomState]string{Running: "Resuming Pom"}, Running: map[PomState]string{Running: "Pom is already running"}, None: map[PomState]string{None: "No Pom to resume"}},
}

type PomMessage struct {
	State           PomState      `json:"state"`
	StartTime       time.Time     `json:"start_time"`
	FinishTime      time.Time     `json:"finish_time"`
	TimeSpentPaused time.Duration `json:"total_time_paused"`
	Message         string        `json:"message"`
	Name            string        `json:"name"`
}

type pomJson PomMessage

func (pm *PomMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		State           string `json:"state"`
		TimeSpentPaused string `json:"total_time_paused"`
		*pomJson
	}{
		State:           pm.State.String(),
		pomJson:         (*pomJson)(pm),
		TimeSpentPaused: pm.TimeSpentPaused.String(),
	})
}

func PrettyTime(d time.Duration) string {
	if d.Minutes() > 1 {
		return "" + strconv.Itoa(int(d.Minutes()))
	} else {
		return "Less than 1 min"
	}
}

type Pom struct {
	startTime time.Time
	pauseTime time.Time
	timer     *time.Timer

	pomLength       time.Duration
	timeSpentPaused time.Duration

	name    string
	running bool
	state   PomState

	done  chan bool
	stop  chan struct{}
	pause chan struct{}
}

func NewPom(optionalName string, duration time.Duration) *Pom {
	return &Pom{
		name:            optionalName,
		state:           None,
		pomLength:       duration,
		timeSpentPaused: 0,
		done:            make(chan bool),
		stop:            make(chan struct{}),
		pause:           make(chan struct{}),
	}
}

func (p *Pom) ToPomMessage(action string, oldState PomState) *PomMessage {
	logger.Printf("action %s, old %s, current %s\n", action, oldState, p.State())
	message := stateMessageLookup[action][oldState][p.State()]
	if p == nil {
		logger.Println("Nil pom state", p.State())
		return &PomMessage{
			State: None,
			// StartTime:       nil,
			TimeSpentPaused: time.Minute * 0,
			// Name:            "",
			// FinishTime:      nil,
			Message: message,
		}
	}
	logger.Println("Start Time", p.startTime)
	logger.Println("Finish Time", p.FinishTime())
	logger.Println("Time Left", p.TimeLeft())
	pomMessage := &PomMessage{
		State:           p.state,
		StartTime:       p.startTime,
		TimeSpentPaused: p.timeSpentPaused,
		Name:            p.name,
		FinishTime:      p.FinishTime(),
		Message:         message,
	}
	if p.State() == Paused {
		pomMessage.TimeSpentPaused += time.Now().Sub(p.pauseTime)
	}
	return pomMessage
}

// FIXME: Not working correctly in Running state
func (p *Pom) FinishTime() time.Time {
	return time.Now().Add(p.TimeLeft())
}

func (p *Pom) TimeLeft() time.Duration {
	timeLeft := p.startTime.Add(p.pomLength).Sub(time.Now())
	if p.State() == Paused {
		timeLeft += time.Now().Sub(p.pauseTime)
	}
	return timeLeft + p.timeSpentPaused
}

func (p *Pom) Start() bool {
	if p.State() == None {
		p.startTime = time.Now()
		p.createTimerFor(p.pomLength)
		return true
	}
	return false
}

func (p *Pom) Stop() bool {
	if p.State() == Running || p.State() == Paused {
		p.timer.Stop()
		p.stop <- struct{}{}
		return true
	}
	return false
}

func (p *Pom) Pause() bool {
	if p.State() == Running {
		p.timer.Stop()
		p.pauseTime = time.Now()
		logger.Println("Paused, time left:", p.TimeLeft())
		p.pause <- struct{}{}
		return true
	}
	return false
}

func (p *Pom) Resume() bool {
	if p.State() == Paused {
		timeLeft := p.TimeLeft()
		logger.Println("Resuming with", timeLeft)
		p.createTimerFor(timeLeft)
		p.timeSpentPaused += time.Now().Sub(p.pauseTime)
		return true
	}
	return false
}

func (p *Pom) State() PomState {
	if p == nil {
		return None
	}
	return p.state
}

func (p *Pom) createTimerFor(duration time.Duration) {
	p.timer = time.NewTimer(duration)
	logger.Println("NewTimer for", duration)
	p.state = Running
	go func() {
		for p.State() != None {
			select {
			case <-p.pause:
				logger.Println("Paused Pom")
				p.state = Paused
			case <-p.timer.C:
				logger.Println("Finished Pom")
				p.state = None
				p.done <- true
			case <-p.stop:
				logger.Println("Stopped Pom")
				p.state = None
				p.done <- false
			}
		}
	}()
}
