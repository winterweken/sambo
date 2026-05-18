package nfs

import "strings"

// BuildOptions constructs a standard NFS options string from boolean parameters.
// This centralizes option building to avoid duplication across CLI and TUI code.
func BuildOptions(readOnly, asyncMode, noRootSquash bool) string {
	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}

	if asyncMode {
		options = append(options, "async")
	} else {
		options = append(options, "sync")
	}

	if noRootSquash {
		options = append(options, "no_root_squash")
	} else {
		options = append(options, "root_squash")
	}

	options = append(options, "no_subtree_check")

	return strings.Join(options, ",")
}
