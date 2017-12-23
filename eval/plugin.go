// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
)

var plugins pluginManager

// pluginManager is a process-global manager of generated plugins.
//
// Completely independent *Program objects co-ordinate the
// plugins they generate to avoid multiple attempts at loading
// the same plugin.
type pluginManager struct {
	mu      sync.Mutex
	tempdir string
}

func (m *pluginManager) init() error {
	if m.tempdir == "" {
		var err error
		m.tempdir, err = ioutil.TempDir("", "ng-tmp-")
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(m.tempdir, "src"), 0775); err != nil {
		return err
	}
	return nil
}

// create creates and loads a single-file plugin outside
// of the process-temporary plugin GOPATH.
func (m *pluginManager) create(name string, contents []byte) (*plugin.Plugin, error) {
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
	cmd := exec.Command("go", "build", "-buildmode=plugin", name)
	cmd.Dir = m.tempdir
	out, err := cmd.CombinedOutput()
	os.Remove(path)
	if err != nil {
		return nil, fmt.Errorf("building plugin %s failed: %v\n%s", name, err, out)
	}
	pluginName := name[:len(name)-3] + ".so"
	plg, err := plugin.Open(filepath.Join(m.tempdir, pluginName))
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", name, err)
	}
	return plg, nil
}

func (m *pluginManager) dir(pkgPath string) (adjPkgPath, dir string, err error) {
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

func (m *pluginManager) open(mainPkgPath string) (*plugin.Plugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.Command("go", "build", "-buildmode=plugin", mainPkgPath)
	cmd.Env = append(os.Environ(), "GOPATH="+m.tempdir)
	cmd.Dir = filepath.Join(m.tempdir, "src", mainPkgPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("building plugin %s failed: %v\n%s", mainPkgPath, err, out)
	}

	pluginName := filepath.Base(mainPkgPath) + ".so"
	filename := filepath.Join(m.tempdir, "src", mainPkgPath, pluginName)
	plg, err := plugin.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", pluginName, err)
	}
	os.Remove(filename)
	return plg, nil
}
