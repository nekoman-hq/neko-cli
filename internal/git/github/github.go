// Package github includes github specific structs
package github

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      19.12.2025
*/

type Release struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	Author      Author `json:"author"`
	PreRelease  bool   `json:"prerelease"`
}
type Author struct {
	Login string `json:"login"`
}

type Tag struct {
	Name   string `json:"name"`
	Commit Commit `json:"commit"`
}

type Commit struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}
