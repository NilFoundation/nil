package common

import (
	"time"
)

type Timer interface {
	Now() uint64
	NowTime() time.Time
}

type RealTimerImpl struct{}

var _ Timer = new(RealTimerImpl)

func (t *RealTimerImpl) Now() uint64 {
	return uint64(time.Now().Unix())
}

func (t *RealTimerImpl) NowTime() time.Time {
	return time.Now().UTC()
}

type TestTimerImpl struct {
	nowTime uint64
}

func (t *TestTimerImpl) Now() uint64 {
	return t.nowTime
}

func (t *TestTimerImpl) NowTime() time.Time {
	return time.Unix(int64(t.nowTime), 0).UTC()
}

func (t *TestTimerImpl) Add(duration time.Duration) {
	t.nowTime += uint64(duration.Seconds())
}

func (t *TestTimerImpl) SetTime(time time.Time) {
	t.nowTime = uint64(time.Unix())
}

var realTimer = RealTimerImpl{}

func NewTimer() *RealTimerImpl {
	return &realTimer
}

func NewTestTimer(nowTime uint64) *TestTimerImpl {
	return &TestTimerImpl{nowTime: nowTime}
}

func NewTestTimerFromTime(nowTime time.Time) *TestTimerImpl {
	return NewTestTimer(uint64(nowTime.Unix()))
}
