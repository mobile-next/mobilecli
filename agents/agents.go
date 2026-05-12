package agents

import _ "embed"

//go:embed android/devicekit.so
var AndroidDevicekitSO []byte

//go:embed android/devicekit.dex
var AndroidDevicekitDEX []byte

//go:embed ios/agent-sim.dylib
var IOSAgentSimDylib []byte
