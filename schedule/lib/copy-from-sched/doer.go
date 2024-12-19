package sched

import (
	"sync"
	"time"
)

type doer struct {
	once sync.Once
	tick chan tick
	done chan struct{}
}

func (d *doer) init() {
	d.once.Do(func() {
		d.tick = make(chan tick, 1)
		d.done = make(chan struct{})
		go d.run()
	})
}

func (d *doer) Exec(now time.Time, list *taskSlice) (ok bool) {
	d.init()
	select {
	case d.tick <- tick{now, list}:
		return true
	default:
		return false
	}
}

func (d *doer) Stop() {
	d.init()
	close(d.tick)
	<-d.done
}

func (d *doer) run() {
	defer close(d.done)
	for t := range d.tick {
		now := t.now
		t.tasks.ForEach(func(t *task) {

			t.exec(now)
		})
	}
}
