// Package github includes github specific structs
package github

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      19.12.2025
*/

type RepoInfo struct {
	Owner string
	Repo  string
}

type Release struct {
	Author      Author  `json:"author"`
	Name        string  `json:"name"`
	TagName     string  `json:"tag_name"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Body        string  `json:"body"`
	Assets      []Asset `json:"assets"`
	PreRelease  bool    `json:"prerelease"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int    `json:"size"`
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
