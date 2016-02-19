package other

import "time"

type PomReminder struct {
	title    string
	message  string
	remindIn time.Duration

	timer *time.Timer
}

func (p *PomReminder) setReminder() {
	p.timer = time.NewTimer(p.remindIn)
	go func() {
		<-p.timer.C
	}()
}
