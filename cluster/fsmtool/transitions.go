package fsmtool

import (
	"fmt"
	"io"
	"log"
	"time"
)

// StateTransitionTable is a tool to manage state transitions.
type StateTransitionTable struct {
	CurrentState     interface{}
	LogOnError       bool
	PanicOnError     bool
	LogStates        bool
	LogTransitions   bool
	ValidTransitions map[interface{}][]interface{}
}

// NewStateTransitionTable creates a new state transition table.
func NewStateTransitionTable(initialState interface{}) *StateTransitionTable {
	return &StateTransitionTable{
		CurrentState:     initialState,
		LogOnError:       false,
		PanicOnError:     false,
		LogTransitions:   false,
		ValidTransitions: make(map[interface{}][]interface{}),
	}
}

// AddTransitions adds a state transition to the table
func (s *StateTransitionTable) AddTransitions(states ...interface{}) bool {
	if len(states)%2 != 0 {
		return false
	}
	for i := 0; i < len(states); i += 2 {
		from := states[i]
		to := states[i+1]
		existing, ok := s.ValidTransitions[from]
		if !ok {
			existing = make([]interface{}, 0)
		}
		for _, v := range existing {
			if v == to {
				return false
			}
		}
		existing = append(existing, to)
		s.ValidTransitions[from] = existing
	}
	return true
}

// SetState sets a new state if it is valid
func (s *StateTransitionTable) SetState(state interface{}) bool {
	validstates, ok := s.ValidTransitions[s.CurrentState]
	if !ok {
		panic(fmt.Sprintf("I'm in an invalid state. There is no transitions from %s to another state (or %s)", s.CurrentState, state))
	}
	for _, v := range validstates {
		if v == state {
			s.CurrentState = state
			return true
		}
	}
	return false
}

// Apply applies the state transition with the given function and logs debug information
func (s *StateTransitionTable) Apply(newState interface{}, code func(stt *StateTransitionTable)) bool {
	if !s.SetState(newState) {
		if s.LogOnError {
			log.Printf("Invalid state assignment: Current state = %s, invalid state = %s", s.CurrentState, newState)
		}
		if s.PanicOnError {
			panic(fmt.Sprintf("Invalid state assignment: Current state = %s, invalid state = %s", s.CurrentState, newState))
		}
		return false
	}
	var start time.Time
	if s.LogTransitions {
		start = time.Now()
	}

	s.CurrentState = newState
	code(s)
	if s.LogTransitions {
		stop := time.Now()
		execTime := float64(stop.Sub(start)) / float64(time.Millisecond)
		log.Printf("State: %s->%s took %f ms", s.CurrentState, newState, execTime)
	}
	return true
}

// DumpTransitions dumps the transitions in a dot-compatible format
func (s *StateTransitionTable) DumpTransitions(writer io.Writer) {
	fmt.Fprintf(writer, "digraph StateTransitions {\n")
	for k, v := range s.ValidTransitions {
		for _, to := range v {
			fmt.Fprintf(writer, "    %s -> %s;\n", k, to)
		}
	}
	fmt.Fprintf(writer, "}\n")
}
