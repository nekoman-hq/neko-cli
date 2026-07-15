package history

func parseHistoryQueryRequest(flags map[string]any) historyQueryRequest {
	request := historyQueryRequest{}
	if unit, ok := flags["unit"].(string); ok {
		request.Unit = unit
	}
	return request
}
