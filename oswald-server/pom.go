package main

import "time"

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
		p.timeSpentPaused += time.Now().Sub(p.pauseTime)
		p.createTimerFor(p.TimeLeft())
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
