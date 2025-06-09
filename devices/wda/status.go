package wda

func GetWebDriverAgentStatus() (map[string]interface{}, error) {
	return GetWebDriverAgentEndpoint("status")
}
