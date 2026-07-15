package contributors

func parseContributorsQueryRequest(flags map[string]any) contributorsQueryRequest {
	request := contributorsQueryRequest{}
	if unit, ok := flags["unit"].(string); ok {
		request.Unit = unit
	}
	return request
}
