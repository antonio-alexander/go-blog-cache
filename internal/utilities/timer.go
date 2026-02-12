package utilities

import (
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

type timer struct {
	stopTime  int64
	startTime int64
}

type timers struct {
	sync.RWMutex
	timers map[string][]*timer
}

type Timers interface {
	Start(group string) int
	Stop(group string, index int) int64
	ReadAll() *data.Timers
	Clear()
}

func NewTimers() Timers {
	return &timers{
		timers: make(map[string][]*timer),
	}
}

func (t *timers) Clear() {
	t.Lock()
	defer t.Unlock()

	t.timers = nil
	t.timers = make(map[string][]*timer)
}

func (t *timers) Start(group string) int {
	t.Lock()
	defer t.Unlock()

	if _, found := t.timers[group]; !found {
		t.timers[group] = make([]*timer, 0, 100)
	}
	t.timers[group] = append(t.timers[group],
		&timer{startTime: time.Now().UnixNano()})
	return len(t.timers[group]) - 1
}

func (t *timers) Stop(group string, index int) int64 {
	t.Lock()
	defer t.Unlock()

	if _, found := t.timers[group]; !found {
		return -1
	}
	if len(t.timers[group])-1 > index {
		return -1
	}
	t.timers[group][index].stopTime = time.Now().UnixNano()
	return t.timers[group][index].stopTime - t.timers[group][index].startTime
}

func (t *timers) ReadAll() *data.Timers {
	t.Lock()
	defer t.Unlock()

	totals, averages := make(map[string]int64), make(map[string]int64)
	for group := range t.timers {
		var total int64
		var offset int

		for _, timer := range t.timers[group] {
			if timer.stopTime <= 0 {
				offset++
				continue
			}
			total += timer.stopTime - timer.startTime
		}
		totals[group] = total
		averages[group] = int64(total / int64(len(t.timers[group])-offset))
	}
	return &data.Timers{
		Totals:   totals,
		Averages: averages,
	}
}
