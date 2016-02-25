package main

import "time"

type PomState int

const (
	Running PomState = iota
	Paused
	None
)

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

func (p *Pom) FinishTime() time.Time {
	return p.startTime.Add(p.TimeLeft())
}

func (p *Pom) TimeLeft() time.Duration {
	timeLeft := p.startTime.Add(p.pomLength).Sub(time.Now())
	if p.State() == Paused {
		timeLeft += time.Now().Sub(p.pauseTime)
	}
	return timeLeft + p.timeSpentPaused
}

func (p *Pom) Start() bool {
	p.state = Running
	p.startTime = time.Now()
	p.createTimerFor(p.pomLength)
	return true
}

func (p *Pom) Stop() bool {
	p.state = None
	p.timer.Stop()

	logger.Println("Stopping Pom", p.name)
	p.stop <- struct{}{}
	return true
}

func (p *Pom) Pause() bool {
	if p.state == Running { // if running then pause
		p.state = Paused
		p.timer.Stop()
		p.pauseTime = time.Now()
		logger.Println("Paused, time left:", p.TimeLeft())
		p.pause <- struct{}{}
		return true
	}
	return false
}

func (p *Pom) Resume() bool {
	if p.state == Paused { // if running then pause
		// get timeSpentPaused
		p.timeSpentPaused += time.Now().Sub(p.pauseTime)
		// figure out duration left and restart timer
		p.createTimerFor(p.TimeLeft())
		p.state = Running
	}
	return false
}

func (p *Pom) State() PomState {
	if p == nil {
		return None
	}
	return p.state
}

// TODO: Move all state changes here
func (p *Pom) createTimerFor(duration time.Duration) {
	p.timer = time.NewTimer(duration)
	go func() {
		select {
		case <-p.pause:
			logger.Println("Paused Pom")
		case <-p.timer.C:
			logger.Println("Finished Pom")
			p.state = None
			p.done <- true
		case <-p.stop:
			logger.Println("Stopped Pom")
			p.done <- false
		}
	}()
}
