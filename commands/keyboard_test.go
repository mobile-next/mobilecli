package commands

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const shortCloseTimeout = 20 * time.Millisecond

type fakeKeyboard struct {
	hideResult         bool
	hideErr            error
	statusErr          error
	failingStatusCalls int
	visibleReports     []bool
	statusCallCount    int
}

func (k *fakeKeyboard) HideKeyboard() (bool, error) {
	return k.hideResult, k.hideErr
}

// IsKeyboardVisible fails for the first failingStatusCalls calls, then walks
// through visibleReports and repeats the last one.
func (k *fakeKeyboard) IsKeyboardVisible() (bool, error) {
	call := k.statusCallCount
	k.statusCallCount++
	if call < k.failingStatusCalls {
		return false, k.statusErr
	}
	index := min(call-k.failingStatusCalls, len(k.visibleReports)-1)
	return k.visibleReports[index], nil
}

func TestHideSucceedsOnceTheKeyboardReportsClosed(t *testing.T) {
	keyboard := &fakeKeyboard{hideResult: true, visibleReports: []bool{true, false}}

	dismissed, err := hideKeyboardAndWaitForClose(keyboard, time.Second)

	require.NoError(t, err)
	assert.True(t, dismissed)
	assert.Equal(t, 2, keyboard.statusCallCount)
}

func TestHideFailsWhenTheKeyboardStaysUp(t *testing.T) {
	keyboard := &fakeKeyboard{hideResult: true, visibleReports: []bool{true}}

	_, err := hideKeyboardAndWaitForClose(keyboard, shortCloseTimeout)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "still visible")
}

func TestHideWithNoKeyboardShowingDoesNotPoll(t *testing.T) {
	keyboard := &fakeKeyboard{hideResult: false, visibleReports: []bool{false}}

	dismissed, err := hideKeyboardAndWaitForClose(keyboard, time.Second)

	require.NoError(t, err)
	assert.False(t, dismissed)
	assert.Zero(t, keyboard.statusCallCount)
}

func TestHideSurvivesOneFailedStatusCheck(t *testing.T) {
	keyboard := &fakeKeyboard{
		hideResult:         true,
		visibleReports:     []bool{true, false},
		statusErr:          errors.New("agent busy"),
		failingStatusCalls: 1,
	}

	dismissed, err := hideKeyboardAndWaitForClose(keyboard, time.Second)

	require.NoError(t, err)
	assert.True(t, dismissed)
}

func TestHideReportsAStatusFailureThatNeverClears(t *testing.T) {
	statusFailure := errors.New("no foreground app")
	keyboard := &fakeKeyboard{
		hideResult:         true,
		visibleReports:     []bool{true},
		statusErr:          statusFailure,
		failingStatusCalls: math.MaxInt,
	}

	_, err := hideKeyboardAndWaitForClose(keyboard, shortCloseTimeout)

	require.ErrorIs(t, err, statusFailure)
}
