package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

const (
	ErrMissingEnvVar = "NEKO_1000"
	ErrNoGitRepo     = "NEKO_1001"
	ErrNoRemote      = "NEKO_1002"
	ErrInvalidRemote = "NEKO_1003"

	ErrAPIRequest  = "NEKO_2000"
	ErrAPIResponse = "NEKO_2001"
	ErrNoReleases  = "NEKO_2002"
)
