package ios

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/mobile-next/mobilecli/internal/utils"
	log "github.com/sirupsen/logrus"
)

var ErrTunnelAlreadyRunning = errors.New("tunnel is already running")

type TunnelManager struct {
	udid         string
	pairRecords  tunnel.PairRecordManager
	tunnelMgr    *tunnel.TunnelManager
	loop         *updateLoop // the running update loop, nil when none
	tunnelCancel context.CancelFunc
	tunnelMutex  sync.Mutex
	updateCtx    context.Context
}

// updateLoop is one generation of the tunnel update loop. exited is set under tunnelMutex,
// so the loop and ResetTunnel agree on which of them closes the loop's manager.
type updateLoop struct {
	done   chan struct{}
	exited bool
}

// GetTunnelManager returns the underlying go-ios tunnel manager
func (tm *TunnelManager) GetTunnelManager() *tunnel.TunnelManager {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()
	return tm.tunnelMgr
}

func NewTunnelManager(udid string) (*TunnelManager, error) {
	// Create secure subdirectory for pair records
	dir := filepath.Join(os.TempDir(), "mobilecli-pairrecords")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create pair records directory: %w", err)
	}

	pm, err := tunnel.NewPairRecordManager(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create pair record manager: %w", err)
	}

	// Create go-ios tunnel manager with userspace TUN enabled
	tunnelMgr := tunnel.NewTunnelManager(pm, true)

	return &TunnelManager{
		udid:        udid,
		pairRecords: pm,
		tunnelMgr:   tunnelMgr,
	}, nil
}

func (tm *TunnelManager) StartTunnel() error {
	return tm.StartTunnelWithCallback(nil)
}

func (tm *TunnelManager) StartTunnelWithCallback(onProcessDied func(error)) error {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.updateCtx != nil {
		return ErrTunnelAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	tm.tunnelCancel = cancel
	tm.updateCtx = ctx

	// the loop works on this generation's manager; ResetTunnel may swap in a new one
	mgr := tm.tunnelMgr
	loop := &updateLoop{done: make(chan struct{})}
	tm.loop = loop

	// Start the tunnel manager update loop for this device
	go func() {
		defer close(loop.done)
		defer func() {
			tm.tunnelMutex.Lock()
			// only clear state that still belongs to this generation
			if tm.updateCtx == ctx {
				tm.updateCtx = nil
				tm.tunnelCancel = nil
			}
			// decided under the lock: either a reset already swapped the manager and left it
			// to us, or we mark the loop exited and the reset closes it
			replaced := tm.tunnelMgr != mgr
			loop.exited = true
			tm.tunnelMutex.Unlock()
			// no tunnel handshake is in flight any more, so this releases the tunnel it built
			if replaced {
				if err := mgr.Close(); err != nil {
					utils.Verbose("Error closing replaced tunnel manager: %v", err)
				}
			}
		}()

		// Initial tunnel update to start tunnels for connected devices
		err := mgr.UpdateTunnels(ctx)
		if err != nil {
			log.WithError(err).Warn("Failed to update tunnels initially")
			if onProcessDied != nil {
				onProcessDied(fmt.Errorf("tunnel manager failed to start: %w", err))
			}
			return
		}

		utils.Verbose("Tunnel manager started for device %s", tm.udid)

		// Keep updating tunnels periodically to handle device connects/disconnects
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := mgr.UpdateTunnels(ctx)
				if err != nil {
					log.WithError(err).Debug("Failed to update tunnels")
				}
			}
		}
	}()

	return nil
}

func (tm *TunnelManager) IsTunnelRunning() bool {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()
	return tm.updateCtx != nil
}

func (tm *TunnelManager) StopTunnel() error {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.updateCtx == nil {
		return fmt.Errorf("no tunnel process running")
	}

	// Cancel the update loop
	if tm.tunnelCancel != nil {
		tm.tunnelCancel()
	}

	// Close the tunnel manager to clean up all tunnels
	err := tm.tunnelMgr.Close()
	if err != nil {
		utils.Verbose("Error closing tunnel manager: %v", err)
	}

	utils.Verbose("Stopping tunnel manager for device %s", tm.udid)
	tm.updateCtx = nil
	tm.tunnelCancel = nil

	return err
}

// GetTunnelInfo returns tunnel information for the specific device
func (tm *TunnelManager) GetTunnelInfo() (*tunnel.Tunnel, error) {
	tunnels, err := tm.tunnelMgr.ListTunnels()
	if err != nil {
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	// Find tunnel for this device
	for _, t := range tunnels {
		if t.Udid == tm.udid {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("tunnel not found for device %s", tm.udid)
}

// ResetTunnel drops the current tunnel so the next StartTunnel builds a new one. go-ios keeps
// a device's tunnel for as long as usbmuxd lists the device on any connection, so a tunnel
// built over USB outlives the cable it ran on; a go-ios manager cannot be reused once closed,
// hence the fresh one.
func (tm *TunnelManager) ResetTunnel() error {
	tm.tunnelMutex.Lock()
	cancel := tm.tunnelCancel
	loop := tm.loop
	tm.tunnelMutex.Unlock()

	// wait outside the lock: the loop takes it on exit. A loop stuck in a lockdown handshake
	// (go-ios does not pass the context down) must not block the rebuild forever.
	if cancel != nil {
		cancel()
	}
	if loop != nil {
		select {
		case <-loop.done:
		case <-time.After(15 * time.Second):
			utils.Verbose("tunnel loop of device %s did not stop in time, it will close its manager on exit", tm.udid)
		}
	}

	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()
	// whether the loop is still running is read under the same lock it uses to decide, so
	// exactly one of us closes the manager; closing mid-handshake would miss the tunnel
	// being built, so a running loop closes it itself once it sees the swap
	if loop == nil || loop.exited {
		if err := tm.tunnelMgr.Close(); err != nil {
			utils.Verbose("Error closing tunnel manager: %v", err)
		}
	}
	tm.updateCtx = nil
	tm.tunnelCancel = nil
	tm.loop = nil
	tm.tunnelMgr = tunnel.NewTunnelManager(tm.pairRecords, true)
	utils.Verbose("Tunnel reset for device %s", tm.udid)
	return nil
}
