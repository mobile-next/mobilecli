package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const shortCloseTimeout = 20 * time.Millisecond

type fakeKeyboard struct {
	hideResult      bool
	hideErr         error
	statusErr       error
	visibleReports  []bool
	statusCallCount int
}

func (k *fakeKeyboard) HideKeyboard() (bool, error) {
	return k.hideResult, k.hideErr
}

// IsKeyboardVisible walks through visibleReports and repeats the last one.
func (k *fakeKeyboard) IsKeyboardVisible() (bool, error) {
	index := min(k.statusCallCount, len(k.visibleReports)-1)
	k.statusCallCount++
	return k.visibleReports[index], k.statusErr
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

func TestHideReturnsAStatusFailureInsteadOfWaiting(t *testing.T) {
	statusFailure := errors.New("no foreground app")
	keyboard := &fakeKeyboard{hideResult: true, visibleReports: []bool{true}, statusErr: statusFailure}

	_, err := hideKeyboardAndWaitForClose(keyboard, time.Second)

	require.ErrorIs(t, err, statusFailure)
	assert.Equal(t, 1, keyboard.statusCallCount)
}
