package utils

// Version is the mobilecli release version. The release workflow rewrites the
// literal below (see .github/workflows/build.yml); local builds stay "dev".
var Version = "dev"

// UserAgent is the User-Agent mobilecli sends on every outbound HTTP and
// WebSocket request to the cloud. Without it Go sends "Go-http-client/2.0",
// which is indistinguishable from every other Go client in the access logs.
func UserAgent() string {
	return "mobilecli/" + Version
}
