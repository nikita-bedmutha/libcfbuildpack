/*
 * Copyright 2018 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	bp "github.com/buildpack/libbuildpack/layers"
	"github.com/buildpack/libbuildpack/stack"
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/buildpack"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/internal"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/libcfbuildpack/logger"
)

// BuildFactory is a factory for creating a test Build.
type BuildFactory struct {
	// Build is the configured build to use.
	Build build.Build

	// Home is the home directory to use.
	Home string

	// Output is the BuildPlan output at termination.
	Output buildplan.BuildPlan

	t *testing.T
}

// AddBuildPlan adds an entry to a build plan.
func (f *BuildFactory) AddBuildPlan(name string, dependency buildplan.Dependency) {
	f.t.Helper()

	if f.Build.BuildPlan == nil {
		f.Build.BuildPlan = make(buildplan.BuildPlan)
	}

	f.Build.BuildPlan[name] = dependency
}

// AddDependency adds a dependency with version 1.0 to the buildpack metadata and copies a fixture into a cached
// dependency layer.
func (f *BuildFactory) AddDependency(id string, fixturePath string) {
	f.t.Helper()
	f.AddDependencyWithVersion(id, "1.0", fixturePath)
}

// AddDependencyWithVersion adds a dependency to the buildpack metadata and copies a fixture into a cached dependency
// layer
func (f *BuildFactory) AddDependencyWithVersion(id string, version string, fixturePath string) {
	f.t.Helper()

	d := f.newDependency(id, version, filepath.Base(fixturePath))
	f.cacheFixture(d, fixturePath)
	f.addDependency(d)
}

func (f *BuildFactory) addDependency(dependency buildpack.Dependency) {
	f.t.Helper()

	if f.Build.Buildpack.Metadata == nil {
		f.Build.Buildpack.Metadata = make(buildpack.Metadata)
	}

	if _, ok := f.Build.Buildpack.Metadata["dependencies"]; !ok {
		f.Build.Buildpack.Metadata["dependencies"] = make([]map[string]interface{}, 0)
	}

	metadata := f.Build.Buildpack.Metadata
	dependencies := metadata["dependencies"].([]map[string]interface{})

	var stacks []interface{}
	for _, stack := range dependency.Stacks {
		stacks = append(stacks, stack)
	}

	var licenses []map[string]interface{}
	for _, license := range dependency.Licenses {
		licenses = append(licenses, map[string]interface{}{
			"type": license.Type,
			"uri":  license.URI,
		})
	}

	metadata["dependencies"] = append(dependencies, map[string]interface{}{
		"id":       dependency.ID,
		"name":     dependency.Name,
		"version":  dependency.Version.Version.Original(),
		"uri":      dependency.URI,
		"sha256":   dependency.SHA256,
		"stacks":   stacks,
		"licenses": licenses,
	})
}

func (f *BuildFactory) cacheFixture(dependency buildpack.Dependency, fixturePath string) {
	f.t.Helper()

	l := f.Build.Layers.Layer(dependency.SHA256)
	if err := helper.CopyFile(fixturePath, filepath.Join(l.Root, dependency.Name)); err != nil {
		f.t.Fatal(err)
	}

	if err := internal.WriteTomlFile(l.Metadata, 0644, map[string]interface{}{"metadata": dependency}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *BuildFactory) newDependency(id string, version string, name string) buildpack.Dependency {
	f.t.Helper()

	return buildpack.Dependency{
		ID:      id,
		Name:    name,
		Version: internal.NewTestVersion(f.t, version),
		SHA256:  hex.EncodeToString(sha256.New().Sum([]byte(id))),
		URI:     fmt.Sprintf("http://localhost/%s", name),
		Stacks:  buildpack.Stacks{f.Build.Stack},
	}
}

// NewBuildFactory creates a new instance of BuildFactory.
func NewBuildFactory(t *testing.T) *BuildFactory {
	t.Helper()

	root := ScratchDir(t, "build")

	f := BuildFactory{Home: filepath.Join(root, "home"), t: t}

	f.Build.Application.Root = filepath.Join(root, "application")
	f.Build.Buildpack.Root = filepath.Join(root, "buildpack")
	f.Build.BuildPlanWriter = func(buildPlan buildplan.BuildPlan) error {
		f.Output = buildPlan
		return nil
	}
	f.Build.Layers = layers.NewLayers(
		bp.Layers{Root: filepath.Join(root, "layers")},
		bp.Layers{Root: filepath.Join(root, "buildpack-cache")},
		logger.Logger{})
	f.Build.Platform.Root = filepath.Join(root, "platform")
	f.Build.Stack = stack.Stack("test-stack")

	return &f
}
