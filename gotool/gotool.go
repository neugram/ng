// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gotool manages access to the Go tool for building packages,
// plugins, and export data for feeding the go/types package.
//
// It maintains a process-wide temporary directory that is used as a
// GOPATH for building ephemeral packages as part of executing the
// Neugram interpreter. It is process-wide because plugins are
// necessarily so, and so maintaining any finer-grained GOPATHs just
// lead to confusion and bugs.
package gotool

import (
	"fmt"
	goimporter "go/importer"
	gotypes "go/types"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"
)

var M = new(Manager)

// Manager is a process-global manager of an ephemeral GOPATH
// used to generate plugins.
//
// Completely independent *Program objects co-ordinate the
// plugins they generate to avoid multiple attempts at loading
// the same plugin.
type Manager struct {
	mu               sync.Mutex
	tempdir          string
	importer         gotypes.Importer
	importerIsGlobal bool // means we are pre Go 1.10
}

func (m *Manager) gocmd(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = m.tempdir
	cmd.Env = append(os.Environ(), "GOPATH="+m.gopath())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gotool: %v: %v\n%s", args, err, out)
	}
	return nil
}

func (m *Manager) gopath() string {
	usr := os.Getenv("GOPATH")
	if usr == "" {
		usr = filepath.Join(os.Getenv("HOME"), "go")
	}
	return fmt.Sprintf("%s%c%s", m.tempdir, filepath.ListSeparator, usr)
}

func (m *Manager) init() error {
	if m.tempdir != "" {
		return nil
	}
	var err error
	m.tempdir, err = ioutil.TempDir("", "ng-tmp-")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(m.tempdir, "src"), 0775); err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			// Prior to Go 1.10, importer.For did not
			// support having a lookup function passed
			// to it, and instead paniced. In that case,
			// we use the default and always "go install".
			m.importer = goimporter.For(runtime.Compiler, nil)
			m.importerIsGlobal = true
		}
	}()

	m.importer = goimporter.For(runtime.Compiler, m.importerLookup)

	return nil
}

func (m *Manager) importerLookup(path string) (io.ReadCloser, error) {
	filename := filepath.Join(m.tempdir, path+".a")
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(filename), 0775)
		if err := m.gocmd("build", "-o="+filename, path); err != nil {
			return nil, err
		}
		f, err = os.Open(filename)
	}
	if err != nil {
		return nil, fmt.Errorf("gotool: %v", err)
	}
	os.Remove(filename)
	return f, nil
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	os.RemoveAll(m.tempdir)
	m.tempdir = ""
}

func (m *Manager) ImportGo(path string) (*gotypes.Package, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.init(); err != nil {
		return nil, err
	}
	if m.importerIsGlobal {
		// Make sure our source '.a' files are fresh.
		if err := m.gocmd("install", path); err != nil {
			return nil, err
		}
	}
	return m.importer.Import(path)
}

// Create creates and loads a single-file plugin outside
// of the process-temporary plugin GOPATH.
func (m *Manager) Create(name string, contents []byte) (*plugin.Plugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.init(); err != nil {
		return nil, err
	}

	name = strings.Replace(name, "/", "_", -1)
	name = strings.Replace(name, "\\", "_", -1)
	var path string
	for i := 0; true; i++ {
		filename := fmt.Sprintf("ng-plugin-%s-%d.go", name, i)
		path = filepath.Join(m.tempdir, filename)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err != nil {
			if os.IsExist(err) {
				continue // pick a different name
			}
			return nil, err
		}
		_, err = f.Write(contents)
		f.Close()
		if err != nil {
			return nil, err
		}
		break
	}

	name = filepath.Base(path)
	if err := m.gocmd("build", "-buildmode=plugin", name); err != nil {
		return nil, err
	}
	pluginName := name[:len(name)-3] + ".so"
	plg, err := plugin.Open(filepath.Join(m.tempdir, pluginName))
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", name, err)
	}
	return plg, nil
}

func (m *Manager) Dir(pkgPath string) (adjPkgPath, dir string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.init(); err != nil {
		return "", "", err
	}

	gopath := m.tempdir
	adjPkgPath = pkgPath
	dir = filepath.Join(gopath, "src", adjPkgPath)
	i := 0
	for {
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			break
		}
		i++
		adjPkgPath = filepath.Join(fmt.Sprintf("p%d", i), pkgPath)
		dir = filepath.Join(gopath, "src", adjPkgPath)
	}
	if err := os.MkdirAll(dir, 0775); err != nil {
		return "", "", err
	}
	return adjPkgPath, dir, nil
}

func (m *Manager) Open(mainPkgPath string) (*plugin.Plugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pluginName := filepath.Base(mainPkgPath) + ".so"
	filename := filepath.Join(m.tempdir, "src", mainPkgPath, pluginName)

	if err := m.gocmd("build", "-buildmode=plugin", "-o="+filename, mainPkgPath); err != nil {
		return nil, err
	}

	plg, err := plugin.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", pluginName, err)
	}
	os.Remove(filename)
	return plg, nil
}
