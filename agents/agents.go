package agents

import _ "embed"

//go:embed android/mobilecli.so
var AndroidMobilecliSO []byte

//go:embed android/mobilecli.dex
var AndroidMobilecliDEX []byte

//go:embed ios/agent-sim.dylib
var IOSAgentSimDylib []byte
