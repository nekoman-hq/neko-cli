package presentation

// Text declares preformatted readable output for content that must remain
// intact outside a table, such as a generated configuration preview. It is
// transport metadata and does not change the response data contract.
type Text struct {
	Content string `json:"content"`
}
