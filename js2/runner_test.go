/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js2

import (
	"context"
	"testing"

	"github.com/loadimpact/k6/js2/common"
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestRunnerNew(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			let counter = 0;
			export default function() { counter++; }
		`),
	}, afero.NewMemMapFs())
	assert.NoError(t, err)

	t.Run("NewVU", func(t *testing.T) {
		vu, err := r.newVU()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), vu.Runtime.Get("counter").Export())

		t.Run("RunOnce", func(t *testing.T) {
			_, err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, int64(1), vu.Runtime.Get("counter").Export())
		})
	})
}

func TestRunnerGetDefaultGroup(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs())
	assert.NoError(t, err)
	assert.NotNil(t, r.GetDefaultGroup())
}

func TestRunnerOptions(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs())
	assert.NoError(t, err)

	assert.Equal(t, r.Bundle.Options, r.GetOptions())
	assert.Equal(t, null.NewBool(false, false), r.Bundle.Options.Paused)
	r.ApplyOptions(lib.Options{Paused: null.BoolFrom(true)})
	assert.Equal(t, r.Bundle.Options, r.GetOptions())
	assert.Equal(t, null.NewBool(true, true), r.Bundle.Options.Paused)
	r.ApplyOptions(lib.Options{Paused: null.BoolFrom(false)})
	assert.Equal(t, r.Bundle.Options, r.GetOptions())
	assert.Equal(t, null.NewBool(false, true), r.Bundle.Options.Paused)
}

func TestVURunContext(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() { fn(); }`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}

	vu, err := r.newVU()
	if !assert.NoError(t, err) {
		return
	}

	fnCalled := false
	vu.Runtime.Set("fn", func() {
		fnCalled = true
		assert.Equal(t, vu.Runtime, common.GetRuntime(vu.ctx), "incorrect runtime in context")
		assert.Equal(t, &common.State{
			Group: r.GetDefaultGroup(),
		}, common.GetState(vu.ctx), "incorrect state in context")
	})
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
	assert.True(t, fnCalled, "fn() not called")
}

func TestVUIntegrationGroups(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		import { group } from "k6";
		export default function() {
			fnOuter();
			group("my group", function() {
				fnInner();
				group("nested group", function() {
					fnNested();
				})
			});
		}
		`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}

	vu, err := r.newVU()
	if !assert.NoError(t, err) {
		return
	}

	fnOuterCalled := false
	fnInnerCalled := false
	fnNestedCalled := false
	vu.Runtime.Set("fnOuter", func() {
		fnOuterCalled = true
		assert.Equal(t, r.GetDefaultGroup(), common.GetState(vu.ctx).Group)
	})
	vu.Runtime.Set("fnInner", func() {
		fnInnerCalled = true
		g := common.GetState(vu.ctx).Group
		assert.Equal(t, "my group", g.Name)
		assert.Equal(t, r.GetDefaultGroup(), g.Parent)
	})
	vu.Runtime.Set("fnNested", func() {
		fnNestedCalled = true
		g := common.GetState(vu.ctx).Group
		assert.Equal(t, "nested group", g.Name)
		assert.Equal(t, "my group", g.Parent.Name)
		assert.Equal(t, r.GetDefaultGroup(), g.Parent.Parent)
	})
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
	assert.True(t, fnOuterCalled, "fnOuter() not called")
	assert.True(t, fnInnerCalled, "fnInner() not called")
	assert.True(t, fnNestedCalled, "fnNested() not called")
}
