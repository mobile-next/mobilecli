package devices

import "fmt"

// HideKeyboard dismisses the keyboard through the agent injected into the
// foreground app. The check comes first, so a call with no keyboard showing
// never attaches LLDB to the app.
func (s *SimulatorDevice) HideKeyboard() (bool, error) {
	visible, err := s.IsKeyboardVisible()
	if err != nil || !visible {
		return false, err
	}

	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return false, fmt.Errorf("hiding the keyboard on a simulator only works in apps you installed: %w", err)
	}
	if _, err := agentRequest(port, "device.io.keyboard.hide", nil); err != nil {
		return false, err
	}
	return true, nil
}
