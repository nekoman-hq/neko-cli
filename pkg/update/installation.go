package update

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var homebrewFormulaRE = regexp.MustCompile(`^[A-Za-z0-9@+_.-]+$`)

type installation struct {
	runningExecutable    string
	symlinkPath          string
	canonicalTarget      string
	targetParent         string
	targetMode           fs.FileMode
	targetOwnerUID       int
	targetOwnerGID       int
	ownerKnown           bool
	targetReadable       bool
	parentCreateAllowed  bool
	parentReplaceAllowed bool
	classification       installationClassification
	manager              string
	managerGuidance      string
}

type installationInspector interface {
	Inspect() (installation, error)
}

type identity struct {
	uid    int
	groups []int
	known  bool
}

type ownerLookup func(path string, info fs.FileInfo) (uid, gid int, ok bool)

type osInstallationInspector struct {
	executable      func() (string, error)
	evalSymlinks    func(string) (string, error)
	lstat           func(string) (fs.FileInfo, error)
	stat            func(string) (fs.FileInfo, error)
	open            func(string) (*os.File, error)
	identity        identity
	owner           ownerLookup
	managerPrefixes []string
}

func newOSInstallationInspector() *osInstallationInspector {
	groups, groupsErr := os.Getgroups()
	currentIdentity := identity{uid: os.Geteuid(), groups: groups, known: groupsErr == nil && os.Geteuid() >= 0}
	return &osInstallationInspector{
		executable:      os.Executable,
		evalSymlinks:    filepath.EvalSymlinks,
		lstat:           os.Lstat,
		stat:            os.Stat,
		open:            os.Open,
		identity:        currentIdentity,
		owner:           platformFileOwner,
		managerPrefixes: defaultHomebrewPrefixes(),
	}
}

func (inspector *osInstallationInspector) Inspect() (installation, error) {
	runningPath, err := inspector.executable()
	if err != nil {
		return installation{}, newUpdateError(errorTargetUnsupported, "cannot determine the running executable", err)
	}
	runningPath = filepath.Clean(runningPath)

	linkInfo, err := inspector.lstat(runningPath)
	if err != nil {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("cannot inspect executable path %s", runningPath), err)
	}
	symlinkPath := ""
	if linkInfo.Mode()&fs.ModeSymlink != 0 {
		symlinkPath = runningPath
	}

	canonicalTarget, err := inspector.evalSymlinks(runningPath)
	if err != nil {
		return installation{}, newUpdateError(
			errorTargetUnsupported,
			fmt.Sprintf("cannot resolve executable path %s; the symlink may be missing or cyclic", runningPath),
			err,
		)
	}
	canonicalTarget = filepath.Clean(canonicalTarget)
	targetInfo, err := inspector.stat(canonicalTarget)
	if err != nil {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("cannot inspect update target %s", canonicalTarget), err)
	}
	if !targetInfo.Mode().IsRegular() {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("update target %s is not a regular file", canonicalTarget), nil)
	}
	targetMode := targetInfo.Mode().Perm()
	if targetMode&0o111 == 0 {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("update target %s has no executable permission bits", canonicalTarget), nil)
	}

	readable := false
	if target, openErr := inspector.open(canonicalTarget); openErr == nil {
		readable = true
		_ = target.Close()
	}
	if !readable {
		return installation{}, newUpdateError(errorTargetUnreadable, fmt.Sprintf("update target %s is not readable", canonicalTarget), nil)
	}

	parent := filepath.Dir(canonicalTarget)
	parentInfo, err := inspector.stat(parent)
	if err != nil {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("cannot inspect target parent %s", parent), err)
	}
	if !parentInfo.IsDir() {
		return installation{}, newUpdateError(errorTargetUnsupported, fmt.Sprintf("target parent %s is not a directory", parent), nil)
	}

	targetUID, targetGID, ownerKnown := inspector.owner(canonicalTarget, targetInfo)
	parentUID, parentGID, parentOwnerKnown := inspector.owner(parent, parentInfo)
	parentWritable := modeAllowsDirectoryMutation(parentInfo.Mode(), parentUID, parentGID, parentOwnerKnown, inspector.identity)

	classification := installationUnknown
	manager := ""
	managerGuidance := ""
	if formula, managed := detectHomebrewInstallation(canonicalTarget, inspector.managerPrefixes); managed {
		classification = installationManagerOwned
		manager = "Homebrew"
		managerGuidance = fmt.Sprintf("brew upgrade %s (or brew reinstall %s)", formula, formula)
	} else if ownerKnown && inspector.identity.known && targetUID == inspector.identity.uid {
		classification = installationUnmanagedUser
	} else if ownerKnown && targetUID == 0 || !parentWritable {
		classification = installationUnmanagedPrivileged
	}

	return installation{
		runningExecutable:    runningPath,
		symlinkPath:          symlinkPath,
		canonicalTarget:      canonicalTarget,
		targetParent:         parent,
		targetMode:           targetMode,
		targetOwnerUID:       targetUID,
		targetOwnerGID:       targetGID,
		ownerKnown:           ownerKnown,
		targetReadable:       readable,
		parentCreateAllowed:  parentWritable,
		parentReplaceAllowed: parentWritable,
		classification:       classification,
		manager:              manager,
		managerGuidance:      managerGuidance,
	}, nil
}

func modeAllowsDirectoryMutation(mode fs.FileMode, uid, gid int, ownerKnown bool, current identity) bool {
	if !current.known {
		return false
	}
	if current.uid == 0 {
		return true
	}
	permissions := mode.Perm()
	if ownerKnown && current.uid == uid {
		return permissions&0o300 == 0o300
	}
	if ownerKnown && containsInt(current.groups, gid) {
		return permissions&0o030 == 0o030
	}
	return permissions&0o003 == 0o003
}

func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func defaultHomebrewPrefixes() []string {
	prefixes := []string{"/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"}
	sort.Strings(prefixes)
	return prefixes
}

func detectHomebrewInstallation(canonicalTarget string, prefixes []string) (string, bool) {
	canonicalTarget = filepath.Clean(canonicalTarget)
	for _, prefix := range prefixes {
		cleanPrefix := filepath.Clean(prefix)
		if resolvedPrefix, err := filepath.EvalSymlinks(cleanPrefix); err == nil {
			cleanPrefix = resolvedPrefix
		}
		cellar := filepath.Join(cleanPrefix, "Cellar")
		relative, err := filepath.Rel(cellar, canonicalTarget)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) < 3 || !homebrewFormulaRE.MatchString(parts[0]) || parts[1] == "" {
			continue
		}
		return parts[0], true
	}
	return "", false
}
