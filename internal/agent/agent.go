package agent

import "time"

type Info struct {
	ID           string    `json:"id"`
	StartedAt    time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Capabilities []string  `json:"capabilities"`
}

type Agent struct {
	id           string
	capabilities []string
	startedAt    time.Time
	lastBeat     time.Time
}

func New(id string, capabilities []string) *Agent {
	return &Agent{
		id:           id,
		capabilities: append([]string{}, capabilities...),
	}
}

func (a *Agent) Start() {
	now := time.Now().UTC()
	a.startedAt = now
	a.lastBeat = now
}

func (a *Agent) Heartbeat() {
	a.lastBeat = time.Now().UTC()
}

func (a *Agent) Info() Info {
	return Info{
		ID:           a.id,
		StartedAt:    a.startedAt,
		LastHeartbeat: a.lastBeat,
		Capabilities: append([]string{}, a.capabilities...),
	}
}
