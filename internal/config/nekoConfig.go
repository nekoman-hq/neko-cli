package config

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

type NekoConfig struct {
	ProjectType   string `json:"projectType"`
	ReleaseSystem string `json:"releaseSystem"`
	Version       string `json:"version"`
}
