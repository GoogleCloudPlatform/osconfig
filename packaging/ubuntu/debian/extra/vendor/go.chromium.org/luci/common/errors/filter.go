// Copyright 2016 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

// Filter examines a supplied error and removes instances of excluded errors
// from it. If the entire supplied error is excluded, Filter will return nil.
//
// If a MultiError is supplied to Filter, it will be recursively traversed, and
// its child errors will be turned into nil if they match the supplied filter.
// If a MultiError has all of its children converted to nil as a result of the
// filter, it will itself be reduced to nil.
func Filter(err error, exclude error, others ...error) error {
	return FilterFunc(err, func(e error) bool {
		if e == exclude {
			return true
		}
		for _, v := range others {
			if e == v {
				return true
			}
		}
		return false
	})
}

// FilterFunc examines a supplied error and removes instances of errors that
// match the supplied filter function. If the entire supplied error is removed,
// FilterFunc will return nil.
//
// If a MultiError is supplied to FilterFunc, it will be recursively traversed,
// and its child errors will be turned into nil if they match the supplied
// filter function. If a MultiError has all of its children converted to nil as
// a result of the filter, it will itself be reduced to nil.
//
// Consqeuently, if err is a MultiError, shouldFilter will be called once with
// err as its value and once for every non-nil error that it contains.
func FilterFunc(err error, shouldFilter func(error) bool) error {
	switch {
	case shouldFilter == nil:
		return err
	case err == nil:
		return nil
	case shouldFilter(err):
		return nil
	}

	if merr, ok := err.(MultiError); ok {
		var lme MultiError
		for i, e := range merr {
			if e != nil {
				e = FilterFunc(e, shouldFilter)
				if e != nil {
					if lme == nil {
						lme = make(MultiError, len(merr))
					}
					lme[i] = e
				}
			}
		}
		if lme == nil {
			return nil
		}
		return lme
	}

	return err
}
