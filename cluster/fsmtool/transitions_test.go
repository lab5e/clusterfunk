package fsmtool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type state int

const (
	initialState state = iota
	oneState
	twoState
	threeState
	fourState
	fourOneState
	fourTwoState
	fourThreeState
)

func (s state) String() string {
	switch s {
	case initialState:
		return "initialState"
	case oneState:
		return "oneState"
	case twoState:
		return "twoState"
	case threeState:
		return "threeState"
	case fourState:
		return "fourState"
	case fourOneState:
		return "fourOneState"
	case fourTwoState:
		return "fourTwoState"
	case fourThreeState:
		return "fourThreeState"
	default:
		panic(fmt.Sprintf("Don't know how to state %d", s))
	}
}
func TestTransitionTable(t *testing.T) {
	assert := require.New(t)

	fsm := NewStateTransitionTable(initialState)
	assert.NotNil(fsm)

	assert.True(
		fsm.AddTransitions(
			initialState, oneState,
			oneState, twoState,
			twoState, threeState,
			threeState, fourState,
			fourState, fourOneState,
			fourState, fourTwoState,
			fourState, fourThreeState,
			fourOneState, oneState,
			fourTwoState, twoState,
			fourThreeState, threeState,
		))

	assert.True(fsm.SetState(oneState))
	assert.True(fsm.SetState(twoState))
	assert.True(fsm.SetState(threeState))
	assert.True(fsm.SetState(fourState))
	assert.True(fsm.SetState(fourOneState))
	assert.True(fsm.SetState(oneState))

	assert.False(fsm.SetState(threeState)) // Should be illegal
	assert.True(fsm.SetState(twoState))

	assert.False(fsm.SetState(twoState)) // Should not have own transition
	assert.True(fsm.SetState(threeState))

	assert.False(fsm.SetState(oneState)) // Should not have own transition
	assert.False(fsm.SetState(fourOneState))
	assert.True(fsm.SetState(fourState))
	assert.True(fsm.SetState(fourTwoState))
	assert.False(fsm.SetState(fourOneState))

}

func TestInvalidTransitions(t *testing.T) {
	assert := require.New(t)

	fsm := NewStateTransitionTable(initialState)
	assert.NotNil(fsm)

	assert.True(fsm.AddTransitions(
		oneState, oneState,
		oneState, twoState,
		twoState, threeState,
		threeState, fourState,
		fourState, oneState,
	))

	assert.False(fsm.AddTransitions(
		oneState, twoState,
	))

	assert.False(fsm.AddTransitions(fourState))
	assert.False(fsm.AddTransitions(fourState, twoState, threeState))
}

func TestDebugState(t *testing.T) {
	assert := require.New(t)

	fsm := NewStateTransitionTable(initialState)
	assert.NotNil(fsm)
	fsm.LogOnError = true
	fsm.LogTransitions = true
	fsm.PanicOnError = false
	assert.True(fsm.AddTransitions(
		initialState, oneState,
		oneState, twoState,
		twoState, threeState,
		threeState, fourState,
		fourState, oneState,
	))

	assert.True(fsm.Apply(oneState, func(stt *StateTransitionTable) { t.Logf("State is %s", stt.CurrentState) }))
	assert.False(fsm.Apply(threeState, func(stt *StateTransitionTable) { t.Logf("State is %s", stt.CurrentState) }))

	fsm.PanicOnError = true
	assert.NotPanics(func() {
		fsm.SetState(twoState)
	})
	assert.Panics(func() {
		fsm.Apply(fourState, func(stt *StateTransitionTable) { t.Logf("State is %s", stt.CurrentState) })
	})
	assert.Panics(func() {
		fsm.Apply(fourState, func(stt *StateTransitionTable) { t.Logf("State is %s", stt.CurrentState) })
	})
}
