//go:build !unix

package main

import "io/fs"

func owner(fs.FileInfo) (uid, gid int, ok bool) {
	return 0, 0, false
}
