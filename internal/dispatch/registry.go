package dispatch

import (
	"sort"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

// SessionStatus describes a live session's current state.
type SessionStatus string

const (
	Working   SessionStatus = "working"
	Running   SessionStatus = "running"
	Repairing SessionStatus = "repairing"
	Failed    SessionStatus = "failed"
)

// Session records one live agent session.
type Session struct {
	Handle, Task, Seat, Backend, Model, Difficulty, Effort, Phase string
	Repo                                                          repo.Repo
	Role                                                          Role
	FallbackAttempted, Decomposition                              bool
	Started                                                       time.Time
	Status                                                        SessionStatus
}

// Add registers a live session and stamps its current attempt start time.
func (d *Dispatcher) Add(session Session) {
	session.Started = d.clock.Now()
	d.sessionsMu.Lock()
	defer d.sessionsMu.Unlock()
	d.sessions[session.Handle] = session
}

// Remove deletes and returns a live session by handle.
func (d *Dispatcher) Remove(handle string) (Session, bool) {
	d.sessionsMu.Lock()
	defer d.sessionsMu.Unlock()
	session, ok := d.sessions[handle]
	if !ok {
		return Session{}, false
	}
	delete(d.sessions, handle)
	return session, true
}

// ActiveCount returns the number of live sessions for role.
func (d *Dispatcher) ActiveCount(role Role) int {
	d.sessionsMu.RLock()
	defer d.sessionsMu.RUnlock()
	count := 0
	for _, session := range d.sessions {
		if session.Role == role {
			count++
		}
	}
	return count
}

// RoleCap returns the concurrent session limit for role.
func (d *Dispatcher) RoleCap(role Role) int {
	switch role {
	case Implementer, Role("fleet"):
		return len(d.config.Fleet.Seats)
	case Designer:
		return len(d.config.Designer.Seats)
	case Repairer, Reviewer:
		return 1
	default:
		return 0
	}
}

// FreeSeat returns the first configured unoccupied seat for role.
func (d *Dispatcher) FreeSeat(role Role) string {
	seats := d.roleSeats(role)
	d.sessionsMu.RLock()
	defer d.sessionsMu.RUnlock()
	for _, seat := range seats {
		occupied := false
		for _, session := range d.sessions {
			if session.Seat == seat {
				occupied = true
				break
			}
		}
		if !occupied {
			return seat
		}
	}
	return ""
}

func (d *Dispatcher) roleSeats(role Role) []string {
	var configured []string
	switch role {
	case Implementer, Role("fleet"):
		configured = make([]string, len(d.config.Fleet.Seats))
		for i, seat := range d.config.Fleet.Seats {
			configured[i] = seat.Name
		}
	case Designer:
		configured = make([]string, len(d.config.Designer.Seats))
		for i, seat := range d.config.Designer.Seats {
			configured[i] = seat.Name
		}
	case Repairer:
		configured = make([]string, len(d.config.Repairer.Seats))
		for i, seat := range d.config.Repairer.Seats {
			configured[i] = seat.Name
		}
	case Reviewer:
		configured = make([]string, len(d.config.Reviewer.Seats))
		for i, seat := range d.config.Reviewer.Seats {
			configured[i] = seat.Name
		}
	}
	return configured
}

// SetStatus updates one live session's status.
func (d *Dispatcher) SetStatus(handle string, status SessionStatus) bool {
	d.sessionsMu.Lock()
	defer d.sessionsMu.Unlock()
	session, ok := d.sessions[handle]
	if !ok {
		return false
	}
	session.Status = status
	d.sessions[handle] = session
	return true
}

// SetPhase updates one live session's phase.
func (d *Dispatcher) SetPhase(handle, phase string) bool {
	d.sessionsMu.Lock()
	defer d.sessionsMu.Unlock()
	session, ok := d.sessions[handle]
	if !ok {
		return false
	}
	session.Phase = phase
	d.sessions[handle] = session
	return true
}

// MarkDecomposition flags one live session as an epic decomposition session.
func (d *Dispatcher) MarkDecomposition(handle string) bool {
	d.sessionsMu.Lock()
	defer d.sessionsMu.Unlock()
	session, ok := d.sessions[handle]
	if !ok {
		return false
	}
	session.Decomposition = true
	d.sessions[handle] = session
	return true
}

// Sessions returns an independent snapshot ordered by attempt start time.
func (d *Dispatcher) Sessions() []Session {
	d.sessionsMu.RLock()
	sessions := make([]Session, 0, len(d.sessions))
	for _, session := range d.sessions {
		sessions = append(sessions, session)
	}
	d.sessionsMu.RUnlock()
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Started.Equal(sessions[j].Started) {
			return sessions[i].Handle < sessions[j].Handle
		}
		return sessions[i].Started.Before(sessions[j].Started)
	})
	return sessions
}

// Idle reports whether no live sessions are registered.
func (d *Dispatcher) Idle() bool {
	d.sessionsMu.RLock()
	defer d.sessionsMu.RUnlock()
	return len(d.sessions) == 0
}
