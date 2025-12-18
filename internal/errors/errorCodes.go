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

	ErrConfigExists    = "NEKO_3000"
	ErrConfigNotExists = "NEKO_3001"
	ErrSurveyCancelled = "NEKO_3002"
	ErrSurveyFailed    = "NEKO_3003"
	ErrConfigMarshal   = "NEKO_3004"
	ErrConfigWrite     = "NEKO_3005"
	ErrConfigRead      = "NEKO_3006"
	ErrInvalidVersion  = "NEKO_3007"
)
