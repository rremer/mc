/*
 * Minio Client (C) 2014, 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"sync"
	"sync/atomic"
	"time"
)

// accounter keeps tabs of ongoing data transfer information.
type accounter struct {
	current int64

	Total        int64
	startTime    time.Time
	startValue   int64
	refreshRate  time.Duration
	currentValue int64
	finishOnce   sync.Once
	isFinished   chan struct{}
}

// Instantiate a new accounter.
func newAccounter(total int64) *accounter {
	acct := &accounter{
		Total:        total,
		startTime:    time.Now(),
		startValue:   0,
		refreshRate:  time.Millisecond * 200,
		isFinished:   make(chan struct{}),
		currentValue: -1,
	}
	go acct.writer()
	return acct
}

// write calculate the final speed.
func (a *accounter) write(current int64) float64 {
	fromStart := time.Now().Sub(a.startTime)
	currentFromStart := current - a.startValue
	if currentFromStart > 0 {
		speed := float64(currentFromStart) / (float64(fromStart) / float64(time.Second))
		return speed
	}
	return 0.0
}

// writer update new accounting data for a specified refreshRate.
func (a *accounter) writer() {
	a.Update()
	for {
		select {
		case <-a.isFinished:
			return
		case <-time.After(a.refreshRate):
			a.Update()
		}
	}
}

// accountStat cantainer for current stats captured.
type accountStat struct {
	Total       int64
	Transferred int64
	Speed       float64
}

// Stat provides current stats captured.
func (a *accounter) Stat() accountStat {
	var acntStat accountStat
	a.finishOnce.Do(func() {
		close(a.isFinished)
		acntStat.Total = a.Total
		acntStat.Transferred = a.current
		acntStat.Speed = a.write(atomic.LoadInt64(&a.current))
	})
	return acntStat
}

// Update update with new values loaded atomically.
func (a *accounter) Update() {
	c := atomic.LoadInt64(&a.current)
	if c != a.currentValue {
		a.write(c)
		a.currentValue = c
	}
}

// Set sets the current value atomically.
func (a *accounter) Set(n int64) *accounter {
	atomic.StoreInt64(&a.current, n)
	return a
}

// Add add to current value atomically.
func (a *accounter) Add(n int64) int64 {
	return atomic.AddInt64(&a.current, n)
}

// Read implements Reader which internally updates current value.
func (a *accounter) Read(p []byte) (n int, err error) {
	n = len(p)
	a.Add(int64(n))
	return
}
