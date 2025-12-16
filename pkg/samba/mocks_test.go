package samba

import (
	"bytes"
	"io/fs"
	"os"
	"sambo/pkg/system"
	"time"
)

// MockFileSystem implements system.FileSystem for testing
type MockFileSystem struct {
	Files       map[string][]byte
	Stats       map[string]fs.FileInfo
	MkdirCalls  []string
	RenameCalls []struct{ Old, New string }
	RemoveCalls []string
	WriteCalls  []struct {
		Path string
		Data []byte
	}
	ChownCalls []struct {
		Path string
		Uid  int
		Gid  int
	}
	ChmodCalls []struct {
		Path string
		Perm fs.FileMode
	}
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files:       make(map[string][]byte),
		Stats:       make(map[string]fs.FileInfo),
		MkdirCalls:  []string{},
		RenameCalls: []struct{ Old, New string }{},
		RemoveCalls: []string{},
		WriteCalls: []struct {
			Path string
			Data []byte
		}{},
		ChownCalls: []struct {
			Path string
			Uid  int
			Gid  int
		}{},
		ChmodCalls: []struct {
			Path string
			Perm fs.FileMode
		}{},
	}
}

func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	if content, ok := m.Files[name]; ok {
		return content, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.Files[name] = data
	m.WriteCalls = append(m.WriteCalls, struct {
		Path string
		Data []byte
	}{name, data})
	return nil
}

func (m *MockFileSystem) Stat(name string) (fs.FileInfo, error) {
	if info, ok := m.Stats[name]; ok {
		return info, nil
	}
	// Default to a basic file info if not explicitly mocked but file exists
	if _, ok := m.Files[name]; ok {
		return &mockFileInfo{name: name}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	m.MkdirCalls = append(m.MkdirCalls, path)
	m.Stats[path] = &mockFileInfo{name: path, isDir: true}
	return nil
}

func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	m.RenameCalls = append(m.RenameCalls, struct{ Old, New string }{oldpath, newpath})
	if content, ok := m.Files[oldpath]; ok {
		m.Files[newpath] = content
		delete(m.Files, oldpath)
		return nil
	}
	return os.ErrNotExist
}

func (m *MockFileSystem) Remove(name string) error {
	m.RemoveCalls = append(m.RemoveCalls, name)
	delete(m.Files, name)
	return nil
}

func (m *MockFileSystem) Chown(name string, uid, gid int) error {
	m.ChownCalls = append(m.ChownCalls, struct {
		Path string
		Uid  int
		Gid  int
	}{name, uid, gid})
	return nil
}

func (m *MockFileSystem) Chmod(name string, perm fs.FileMode) error {
	m.ChmodCalls = append(m.ChmodCalls, struct {
		Path string
		Perm fs.FileMode
	}{name, perm})
	return nil
}

func (m *MockFileSystem) OpenFile(name string, flag int, perm os.FileMode) (system.File, error) {
	return &MockFile{buffer: new(bytes.Buffer)}, nil
}

// MockFile implements system.File
type MockFile struct {
	buffer *bytes.Buffer
}

func (m *MockFile) Close() error                            { return nil }
func (m *MockFile) Read(p []byte) (n int, err error)        { return m.buffer.Read(p) }
func (m *MockFile) Write(p []byte) (n int, err error)       { return m.buffer.Write(p) }
func (m *MockFile) WriteString(s string) (n int, err error) { return m.buffer.WriteString(s) }

// MockCommandExecutor implements system.CommandExecutor for testing
type MockCommandExecutor struct {
	RunCalls []struct {
		Name string
		Args []string
	}
	LookPathCalls []string
	MockLookPath  func(file string) (string, error)
	MockRun       func(name string, args ...string) error
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		RunCalls: []struct {
			Name string
			Args []string
		}{},
		LookPathCalls: []string{},
	}
}

func (m *MockCommandExecutor) Run(name string, args ...string) error {
	m.RunCalls = append(m.RunCalls, struct {
		Name string
		Args []string
	}{name, args})
	if m.MockRun != nil {
		return m.MockRun(name, args...)
	}
	return nil
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	m.LookPathCalls = append(m.LookPathCalls, file)
	if m.MockLookPath != nil {
		return m.MockLookPath(file)
	}
	return "/usr/bin/" + file, nil
}

// local mock for fs.FileInfo
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 100 }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// MockPlatform implements system.Platform for testing
type MockPlatform struct {
	IsLinuxReturn         bool
	IsMacOSReturn         bool
	SambaConfigPathReturn string
	SambaConfigDirReturn  string
	ServiceManagerReturn  string
}

func (m *MockPlatform) IsLinux() bool {
	return m.IsLinuxReturn
}

func (m *MockPlatform) IsMacOS() bool {
	return m.IsMacOSReturn
}

func (m *MockPlatform) SambaConfigPath() string {
	if m.SambaConfigPathReturn != "" {
		return m.SambaConfigPathReturn
	}
	return "/etc/samba/smb.conf"
}

func (m *MockPlatform) SambaConfigDir() string {
	if m.SambaConfigDirReturn != "" {
		return m.SambaConfigDirReturn
	}
	return "/etc/samba"
}

func (m *MockPlatform) ServiceManager() string {
	if m.ServiceManagerReturn != "" {
		return m.ServiceManagerReturn
	}
	return "systemctl"
}

// MockAvahiManager implements AvahiManager for testing
type MockAvahiManager struct {
	AddTimeMachineShareCalls []struct {
		ShareName      string
		ExistingShares []string
	}
	RemoveTimeMachineShareCalls []struct {
		ShareName       string
		RemainingShares []string
	}
}

func (m *MockAvahiManager) AddTimeMachineShare(shareName string, existingShares []string) error {
	m.AddTimeMachineShareCalls = append(m.AddTimeMachineShareCalls, struct {
		ShareName      string
		ExistingShares []string
	}{shareName, existingShares})
	return nil
}

func (m *MockAvahiManager) RemoveTimeMachineShare(shareName string, remainingShares []string) error {
	m.RemoveTimeMachineShareCalls = append(m.RemoveTimeMachineShareCalls, struct {
		ShareName       string
		RemainingShares []string
	}{shareName, remainingShares})
	return nil
}
