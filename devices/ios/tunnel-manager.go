package ios

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/mobile-next/mobilecli/utils"
	log "github.com/sirupsen/logrus"
)

type TunnelManager struct {
	udid         string
	tunnelMgr    *tunnel.TunnelManager
	tunnelCancel context.CancelFunc
	tunnelMutex  sync.Mutex
	updateCtx    context.Context
}

// GetTunnelManager returns the underlying go-ios tunnel manager
func (tm *TunnelManager) GetTunnelManager() *tunnel.TunnelManager {
	return tm.tunnelMgr
}

func NewTunnelManager(udid string) (*TunnelManager, error) {
	// Always use temp directory for pair records
	pm, err := tunnel.NewPairRecordManager(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("failed to create pair record manager: %w", err)
	}

	// Create go-ios tunnel manager with userspace TUN enabled
	tunnelMgr := tunnel.NewTunnelManager(pm, true)

	return &TunnelManager{
		udid:      udid,
		tunnelMgr: tunnelMgr,
	}, nil
}

func (tm *TunnelManager) StartTunnel() error {
	return tm.StartTunnelWithCallback(nil)
}

func (tm *TunnelManager) StartTunnelWithCallback(onProcessDied func(error)) error {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.updateCtx != nil {
		return fmt.Errorf("tunnel is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	tm.tunnelCancel = cancel
	tm.updateCtx = ctx

	// Start the tunnel manager update loop for this device
	go func() {
		defer func() {
			tm.tunnelMutex.Lock()
			tm.updateCtx = nil
			tm.tunnelCancel = nil
			tm.tunnelMutex.Unlock()
		}()

		// Initial tunnel update to start tunnels for connected devices
		err := tm.tunnelMgr.UpdateTunnels(ctx)
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
				err := tm.tunnelMgr.UpdateTunnels(ctx)
				if err != nil {
					log.WithError(err).Debug("Failed to update tunnels")
				}
			}
		}
	}()

	return nil
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
