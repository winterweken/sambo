package system

import (
	"io"
	"os"
)

// FileSystem defines the interface for file system operations
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
	Chown(name string, uid, gid int) error
	Chmod(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (File, error)
}

// File represents an open file
type File interface {
	io.Closer
	io.Reader
	io.Writer
	io.StringWriter
}

// RealFileSystem implements FileSystem using the os package
type RealFileSystem struct{}

// ReadFile reads the named file and returns the contents
func (fs *RealFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile writes data to the named file
func (fs *RealFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// Stat returns a FileInfo describing the named file
func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// MkdirAll creates a directory named path
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Rename renames (moves) oldpath to newpath
func (fs *RealFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Remove removes the named file or (empty) directory
func (fs *RealFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// Chown changes the numeric uid and gid of the named file
func (fs *RealFileSystem) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

// Chmod changes the mode of the named file
func (fs *RealFileSystem) Chmod(name string, perm os.FileMode) error {
	return os.Chmod(name, perm)
}

// OpenFile opens the named file with specified flag
func (fs *RealFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}
