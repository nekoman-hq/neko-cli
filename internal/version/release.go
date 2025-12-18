package version

type GithubRelease struct {
	Name        string       `json:"name"`
	TagName     string       `json:"tag_name"`
	PublishedAt string       `json:"published_at"`
	PreRelease  bool         `json:"prerelease"`
	Author      GithubAuthor `json:"author"`
	HTMLURL     string       `json:"html_url"`
	Body        string       `json:"body"`
}

type GithubAuthor struct {
	Login string `json:"login"`
}
