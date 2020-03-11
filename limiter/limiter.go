package limiter

import (
	"sync"
	"time"
)

type Limiter struct {
	PeriodMillisecond    int64
	Limit                uint8
	used                 uint8
	active               uint8
	firstTimestampMilSec int64
	turn                 []func()
	inTern               bool
	mutex                sync.Mutex
}

func (l *Limiter) Wait(fn func()) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.inTern {
		l.turn = append(l.turn, fn)
		return
	}

	if l.used >= l.Limit {
		l.turn = append(l.turn, fn)

		if l.active == 0 {
			l.awaitIfNeed()
		}
		return
	}

	if l.used == 0 {
		l.firstTimestampMilSec = time.Now().UnixNano() / 1_000_000
	}
	l.active++
	l.used++

	go func() {
		fn()

		l.mutex.Lock()
		defer l.mutex.Unlock()
		l.active--

		if l.active == 0 && len(l.turn) > 0 {
			l.awaitIfNeed()
		}
	}()
}

func (l *Limiter) awaitIfNeed() {
	timeNow := time.Now().UnixNano() / 1_000_000
	timeAfter := l.firstTimestampMilSec + l.PeriodMillisecond

	if timeNow < timeAfter {
		l.inTern = true
		go l.awaitAndUseTurn(time.Duration(timeAfter - timeNow))
		return
	} else {
		l.useTurn()
	}
}

func (l *Limiter) awaitAndUseTurn(waitMillisecond time.Duration) {
	time.Sleep(time.Millisecond * (waitMillisecond + 10))

	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.useTurn()
}

func (l *Limiter) useTurn() {
	l.firstTimestampMilSec = time.Now().UnixNano() / 1_000_000
	l.used = 0
	group := sync.WaitGroup{}
	turnLen := len(l.turn)
	lastIndex := turnLen - 1
	lastUseIndex := lastIndex

	if turnLen > 100 {
		// TODO warning
	}

	limit := int(l.Limit)
	if lastIndex > limit - 1 {
		lastUseIndex = limit - 1
		group.Add(limit)
	} else {
		group.Add(turnLen)
	}

	for i, fnItem := range l.turn {
		go func(fn func()) {
			fn()
			group.Done()
		}(fnItem)

		if i == lastUseIndex {
			break
		}
	}

	group.Wait()
	l.turn = append(l.turn[lastUseIndex+1:])

	l.inTern = len(l.turn) > 0
	if l.inTern {
		go l.awaitAndUseTurn(time.Duration(l.PeriodMillisecond))
	}
}
