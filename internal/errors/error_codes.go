package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

const (
	ErrMissingEnvVar    = "NEKO_1000"
	ErrNoGitRepo        = "NEKO_1001"
	ErrNoRemote         = "NEKO_1002"
	ErrInvalidRemote    = "NEKO_1003"
	ErrDirtyWorkingTree = "NEKO_1004"
	ErrWrongBranch      = "NEKO_1005"
	ErrDetachedHead     = "NEKO_1006"
	ErrNoUpstream       = "NEKO_1007"
	ErrBranchBehind     = "NEKO_1008"

	ErrAPIRequest  = "NEKO_2000"
	ErrAPIResponse = "NEKO_2001"
	ErrNoReleases  = "NEKO_2002"
	ErrFileAccess  = "NEKO_2003"

	ErrConfigExists     = "NEKO_3000"
	ErrConfigNotExists  = "NEKO_3001"
	ErrSurveyCancelled  = "NEKO_3002"
	ErrSurveyFailed     = "NEKO_3003"
	ErrConfigMarshal    = "NEKO_3004"
	ErrConfigWrite      = "NEKO_3005"
	ErrConfigRead       = "NEKO_3006"
	ErrVersionViolation = "NEKO_3007"

	ErrInvalidReleaseType   = "NEKO_4000"
	ErrInvalidReleaseSystem = "NEKO_4001"
	ErrReleaseFailed        = "NEKO_4002"
	ErrReleaseCommit        = "NEKO_4003"
	ErrReleaseTag           = "NEKO_4004"
	ErrReleasePush          = "NEKO_4005"
	ErrGoReleaserExecution  = "NEKO_4006"
	ErrDependencyMissing    = "NEKO_4007"
	ErrReleaseSystemInit    = "NEKO_4008"
)
