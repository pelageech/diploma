package config

import (
	"errors"
	sched "github.com/pelageech/diploma/schedule/lib/copy-from-sched"
	"github.com/pelageech/diploma/schedule/lib/ingoroutine"
	igc "github.com/pelageech/diploma/schedule/lib/ingoroutine-cancellabe"
	ste "github.com/pelageech/diploma/schedule/lib/simple-time-effective"
	swe "github.com/pelageech/diploma/schedule/lib/simple-with-context"
	workerpool "github.com/pelageech/diploma/schedule/worker-pool"
	"github.com/pelageech/diploma/stand"
	"gopkg.in/yaml.v3"
	"io"
	"time"
)

type Version string

const V1 = Version("v1")

//go:generate stringer -type SchedulerType
type SchedulerType int

const (
	Clock SchedulerType = iota
	MultiClock
	SimpleTimeEffective
	SimpleWithContext
	InGoroutine
	InGoroutineCancellable
	WorkerPool
)

func (s *SchedulerType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var ss string
	if err := unmarshal(&ss); err != nil {
		return err
	}
	switch ss {
	case "clock":
		*s = Clock
	case "multi-clock", "mclock", "multi":
		*s = MultiClock
	case "simple-time-effective", "ste":
		*s = SimpleTimeEffective
	case "simple-context", "sctx":
		*s = SimpleWithContext
	case "in-go":
		*s = InGoroutine
	case "in-go-ctx":
		*s = InGoroutineCancellable
	case "worker-pool":
		*s = WorkerPool
	}
	return nil
}

func ToScheduler(t SchedulerType, params ...any) (stand.Scheduler, error) {
	switch t {
	case Clock:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}

		return sched.NewScheduler(jobs)
	case MultiClock:
		return nil, errors.New("multi-clock not yet supported")
	case SimpleTimeEffective:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}
		return ste.NewScheduler(jobs), nil
	case SimpleWithContext:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}
		return swe.NewScheduler(jobs), nil
	case InGoroutine:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}
		return ingoroutine.NewScheduler(jobs), nil
	case InGoroutineCancellable:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}
		return igc.NewScheduler(jobs), nil
	case WorkerPool:
		if len(params) < 1 {
			return nil, errors.New("must provide one parameter")
		}
		jobs, ok := params[0].([]*stand.Job)
		if !ok {
			return nil, errors.New("invalid parameter")
		}
		return workerpool.NewScheduler(jobs), nil
	}
	return nil, errors.New("unknown SchedulerType")
}

type TargetConfig struct {
	Sleep    time.Duration `yaml:"sleep"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Count    int           `yaml:"count"`
}

type Scheduler struct {
	Type    SchedulerType  `yaml:"type"`
	Targets []TargetConfig `yaml:"targets"`
}

type Config struct {
	Version   Version   `yaml:"version"`
	Scheduler Scheduler `yaml:"scheduler"`
}

func Export(r io.Reader) (*Config, error) {
	var cfg Config
	if err := yaml.NewDecoder(r).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
