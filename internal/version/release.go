package version

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

type GithubRelease struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	PreRelease  string `json:"pre_release"`
	Author      string `json:"author.login"`
	HTMLURL     string `json:"html_url"`
}
