package check

import (
	"time"

	"github.com/suzhc/px-health/internal/link"
)

const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

type Step struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type Result struct {
	Name       string    `json:"name,omitempty"`
	Protocol   string    `json:"protocol,omitempty"`
	Server     string    `json:"server,omitempty"`
	Port       int       `json:"port,omitempty"`
	Steps      []Step    `json:"steps"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	LatencyMS  int64     `json:"latency_ms,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Err        error     `json:"-"`
	Error      string    `json:"error,omitempty"`
}

func NewResult(node link.Node) Result {
	now := time.Now()
	return Result{
		Name:      node.Name,
		Protocol:  node.Protocol,
		Server:    node.Server,
		Port:      node.Port,
		Status:    StatusFailed,
		StartedAt: now,
	}
}

func NewFailedResult(reason string, err error) Result {
	now := time.Now()
	result := Result{
		Status:     StatusFailed,
		Reason:     reason,
		StartedAt:  now,
		FinishedAt: now,
		Err:        err,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (r *Result) addStep(name, status, detail string, duration time.Duration) Step {
	step := Step{Name: name, Status: status, Detail: detail}
	if duration > 0 {
		step.Duration = duration.Round(time.Millisecond).String()
	}
	r.Steps = append(r.Steps, step)
	return step
}

func (r *Result) fail(reason string, err error) Result {
	r.Status = StatusFailed
	r.Reason = reason
	r.Err = err
	if err != nil {
		r.Error = err.Error()
	}
	r.FinishedAt = time.Now()
	return *r
}

func (r *Result) ok(latency time.Duration) Result {
	r.Status = StatusOK
	r.Reason = ""
	r.LatencyMS = latency.Milliseconds()
	r.FinishedAt = time.Now()
	return *r
}
