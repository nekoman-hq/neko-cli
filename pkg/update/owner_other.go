//go:build !unix

package update

import "io/fs"

func platformFileOwner(_ string, _ fs.FileInfo) (int, int, bool) {
	return 0, 0, false
}
