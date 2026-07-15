package validate

func parseValidationQueryRequest(flags map[string]any) validationQueryRequest {
	request := validationQueryRequest{}
	if show, ok := flags["show"].(bool); ok {
		request.Show = show
	}
	if unit, ok := flags["unit"].(string); ok {
		request.Unit = unit
	}
	return request
}
