//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// Package e2etester is a library for VM-based e2e tests using Daisy workflows.
package test_suite_base

import (
	"fmt"
	"reflect"

	e2etestcompute "github.com/GoogleCloudPlatform/osconfig/e2etester/compute"
)

func RunAllTests(tests interface{}) error {
	md, err := e2etestcompute.GetMetadata()
	if err != nil {
		return err
	}
	testname, ok := md["TestName"]
	if !ok {
		return fmt.Errorf("TestName not in metadata")
	}

	typ := reflect.TypeOf(tests)
	m, ok := typ.MethodByName(testname)
	if !ok {
		return fmt.Errorf("could not locate specified test %q", testname)
	}
	if m.Type.NumIn() != 1 {
		return fmt.Errorf("func %q should only have one input", testname)
	}
	if m.Type.In(0) != typ {
		return fmt.Errorf("func %q should take %v type as first arg, but takes %v", testname, typ, m.Type.In(0))
	}
	if m.Type.NumOut() != 1 {
		return fmt.Errorf("func %q should only have one output", testname)
	}
	errType := reflect.TypeOf((*error)(nil)).Elem() // create pointer to nil error then resolve.
	if !m.Type.Out(0).Implements(errType) {
		return fmt.Errorf("func %q should return %v/%T, but returns %v/%T", testname, errType, errType, m.Type.Out(0), m.Type.Out(0))
	}

	res := m.Func.Call([]reflect.Value{reflect.ValueOf(tests)})
	ierr := res[0].Interface()
	if ierr != nil {
		return ierr.(error)
	}
	return nil
}
