package common

import (
	"lsp-gateway/src/utils"
)

// URIToFilePath converts a file:// URI to a file system path
// Delegates to utils.URIToFilePath for backward compatibility
func URIToFilePath(uri string) string {
	return utils.URIToFilePath(uri)
}

// FilePathToURI converts a file system path to a file:// URI
// Delegates to utils.FilePathToURI for backward compatibility
func FilePathToURI(path string) string {
	return utils.FilePathToURI(path)
}
