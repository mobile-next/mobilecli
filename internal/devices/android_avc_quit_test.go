package devices

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// quittingServer plays one running AvcServer for stopAvcServer: whether it
// answers screencapture.stop, and whether it then exits.
type quittingServer struct {
	exited       chan struct{}
	quitErr      error // what screencapture.stop returns
	quitHangs    bool  // screencapture.stop never answers
	exitsOnQuit  bool  // the server exits after answering
	quitRequests atomic.Int32
	nameKills    atomic.Int32
	hostKills    atomic.Int32
}

func newQuittingServer() *quittingServer {
	return &quittingServer{exited: make(chan struct{})}
}

func (q *quittingServer) exit() {
	select {
	case <-q.exited:
	default:
		close(q.exited)
	}
}

func (q *quittingServer) stop() avcServerStop {
	return avcServerStop{
		askToQuit: func() error {
			q.quitRequests.Add(1)
			if q.quitHangs {
				select {}
			}
			if q.quitErr == nil && q.exitsOnQuit {
				go q.exit()
			}
			return q.quitErr
		},
		killByName: func() { q.nameKills.Add(1) },
		killHost: func() {
			q.hostKills.Add(1)
			q.exit()
		},
		exited:     q.exited,
		askTimeout: 50 * time.Millisecond,
		quitWait:   50 * time.Millisecond,
	}
}

func stopWithin(t *testing.T, s avcServerStop) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		stopAvcServer(s)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stopping the server did not return")
	}
}

func TestAServerThatQuitsWhenAskedIsNotKilled(t *testing.T) {
	server := newQuittingServer()
	server.exitsOnQuit = true

	stopWithin(t, server.stop())

	assert.Equal(t, int32(1), server.quitRequests.Load())
	assert.Equal(t, int32(0), server.nameKills.Load(), "no kill by name after a clean quit")
	assert.Equal(t, int32(0), server.hostKills.Load())
}

func TestAServerWithoutTheStopMethodIsKilledByName(t *testing.T) {
	// an older agent answers "method not found"
	server := newQuittingServer()
	server.quitErr = errors.New("Method not found: screencapture.stop")

	stopWithin(t, server.stop())

	assert.Equal(t, int32(1), server.nameKills.Load())
	assert.Equal(t, int32(1), server.hostKills.Load())
}

func TestAServerWhoseStopRequestHangsIsKilledByName(t *testing.T) {
	server := newQuittingServer()
	server.quitHangs = true

	stopWithin(t, server.stop())

	assert.Equal(t, int32(1), server.nameKills.Load())
	assert.Equal(t, int32(1), server.hostKills.Load())
}

func TestAServerThatAnswersButDoesNotExitIsKilledByName(t *testing.T) {
	server := newQuittingServer()

	stopWithin(t, server.stop())

	assert.Equal(t, int32(1), server.nameKills.Load())
	assert.Equal(t, int32(1), server.hostKills.Load())
}

func TestAServerThatIsAlreadyGoneIsLeftAlone(t *testing.T) {
	server := newQuittingServer()
	server.exit()

	stopWithin(t, server.stop())

	assert.Equal(t, int32(0), server.quitRequests.Load())
	assert.Equal(t, int32(0), server.nameKills.Load())
	assert.Equal(t, int32(0), server.hostKills.Load())
}
