package update

import (
	"fmt"
	"io/fs"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
	"golang.org/x/mod/semver"
)

// Action describes the immutable decision made before update side effects begin.
type Action string

const (
	ActionAlreadyCurrent   Action = "already-current"
	ActionUpgrade          Action = "upgrade"
	ActionForcedReinstall  Action = "forced-reinstall"
	ActionInstalledNewer   Action = "installed-newer"
	ActionDowngradeRefused Action = "downgrade-refused"
	ActionUnsupported      Action = "unsupported"
)

type installationClassification string

const (
	installationUnmanagedUser       installationClassification = "unmanaged-user"
	installationUnmanagedPrivileged installationClassification = "unmanaged-privileged"
	installationManagerOwned        installationClassification = "manager-owned"
	installationUnknown             installationClassification = "unknown"
)

type platform struct {
	OS   string
	Arch string
}

// updatePlan is built without downloading or mutating the filesystem. Callers
// pass it by value and do not modify it after construction.
type updatePlan struct {
	installedVersion     string
	selectedVersion      string
	action               Action
	runningExecutable    string
	symlinkPath          string
	canonicalTarget      string
	targetParent         string
	classification       installationClassification
	manager              string
	managerGuidance      string
	refusalReason        string
	guidance             string
	asset                github.Asset
	checksumAsset        github.Asset
	targetOwnerUID       int
	targetOwnerGID       int
	targetMode           fs.FileMode
	downloadRequired     bool
	replacementPermitted bool
	ownerKnown           bool
}

func (plan updatePlan) withRefusal(reason, guidance string) updatePlan {
	plan.refusalReason = reason
	plan.guidance = guidance
	plan.replacementPermitted = false
	return plan
}

func (plan updatePlan) withAssets(assets releaseAssets) updatePlan {
	plan.asset = assets.archive
	plan.checksumAsset = assets.checksum
	return plan
}

func classifyUpdateAction(installed, available string, force bool) (Action, error) {
	installedVersion := normalizeVersion(installed)
	availableVersion := normalizeVersion(available)
	if !semver.IsValid(installedVersion) {
		return ActionUnsupported, fmt.Errorf("installed version %q is not valid semantic versioning", installed)
	}
	if !semver.IsValid(availableVersion) {
		return ActionUnsupported, fmt.Errorf("selected release version %q is not valid semantic versioning", available)
	}

	switch comparison := semver.Compare(installedVersion, availableVersion); {
	case comparison < 0:
		return ActionUpgrade, nil
	case comparison == 0 && force:
		return ActionForcedReinstall, nil
	case comparison == 0:
		return ActionAlreadyCurrent, nil
	case force:
		return ActionDowngradeRefused, fmt.Errorf(
			"installed version %s is newer than selected latest version %s; force is not a downgrade flag",
			installed,
			available,
		)
	default:
		return ActionInstalledNewer, nil
	}
}
