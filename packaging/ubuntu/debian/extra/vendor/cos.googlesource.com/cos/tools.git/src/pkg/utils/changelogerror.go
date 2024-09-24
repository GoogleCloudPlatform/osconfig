// Copyright 2020 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This package contains the error interface returned by changelog and
// findbuild packages. It includes functions to retrieve HTTP status codes
// from Gerrit and Gitiles errors, and functions to create ChangelogErrors
// relevant to the changelog and findbuild features.

package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	grpcCodeToHTTP = map[string]string{
		codes.Unknown.String():            "500",
		codes.InvalidArgument.String():    "400",
		codes.NotFound.String():           "404",
		codes.PermissionDenied.String():   "403",
		codes.Unauthenticated.String():    "401",
		codes.ResourceExhausted.String():  "429",
		codes.FailedPrecondition.String(): "400",
		codes.OutOfRange.String():         "400",
		codes.Internal.String():           "500",
		codes.Unavailable.String():        "503",
		codes.DataLoss.String():           "500",
	}

	// ForbiddenError is a ChangelogError object indicating the user does not have
	// permission to access a resource
	ForbiddenError = &UtilChangelogError{
		httpCode: "403",
		header:   "No Access",
		err:      "This account does not have access to internal repositories. Please retry with an authorized account, or select the external button to query from publically accessible sources.",
	}

	// InternalServerError is a ChangelogError object indicating an internal error
	InternalServerError = &UtilChangelogError{
		httpCode: "500",
		header:   "Internal Server Error",
		err:      "An unexpected error occurred while retrieving the requested information.",
	}

	gitiles403ErrMsg = "unexpected HTTP 403 from Gitiles"
	gerritErrCodeRe  = regexp.MustCompile("status code\\s*(\\d+)")
)

// ChangelogError is the error type used by the changelog and findbuild package
type ChangelogError interface {
	error
	HTTPCode() string
	Header() string
	HTMLError() string
	Retryable() bool
}

// UtilChangelogError implements the ChangelogError interface
type UtilChangelogError struct {
	httpCode  string
	header    string
	err       string
	htmlErr   string
	retryable bool
}

// HTTPCode retrieves the HTTP error code associated with the error
// ex. 400
func (e *UtilChangelogError) HTTPCode() string {
	return e.httpCode
}

// Header retrieves the full HTTP status associated with the error
// ex. 400 Bad Request
func (e *UtilChangelogError) Header() string {
	return e.header
}

// Error returns the error string
func (e *UtilChangelogError) Error() string {
	return e.err
}

// HTMLError returns an HTML version of the error string
func (e *UtilChangelogError) HTMLError() string {
	if e.htmlErr != "" {
		return e.htmlErr
	}
	return e.Error()
}

// Retryable indicates whether increasing the search range
// could resolve this error
func (e *UtilChangelogError) Retryable() bool {
	return e.retryable
}

func unwrapError(err error) error {
	innerErr := err
	for errors.Unwrap(innerErr) != nil {
		innerErr = errors.Unwrap(innerErr)
	}
	return innerErr
}

func croslandLink(croslandURL, source, target string) string {
	return fmt.Sprintf("%s/log/%s..%s", croslandURL, source, target)
}

// BothBuildsNotFound indicates that neither build was not found
func BothBuildsNotFound(croslandURL, source, target, sourceBuildNum, targetBuildNum string) *UtilChangelogError {
	return &UtilChangelogError{
		httpCode: "404",
		header:   "Build Not Found",
		err: strings.Join([]string{
			"The builds associated with input",
			source,
			"and",
			target,
			"cannot be found. It may be possible that the inputs are either invalid or both belong",
			"to pre-Cusky builds. If both of the inputs belong to pre-Cusky builds, note that this tool only supports changelogs",
			"between Cusky builds. Otherwise, please input valid build numbers (example: 13310.1035.0) or valid image names",
			"(example: cos-rc-85-13310-1034-0).",
		}, " "),
		htmlErr: fmt.Sprintf("%s %s and %s %s<br><br>%s %s <a href=%s target=\"_blank\">%s</a>. %s %s",
			"The builds associated with input",
			source,
			target,
			"could not be found.",
			"It may be possible that the inputs are either invalid or both belong to pre-Cusky builds.",
			"If both of the inputs belong to pre-Cusky builds, please check",
			croslandLink(croslandURL, sourceBuildNum, targetBuildNum),
			croslandLink(croslandURL, sourceBuildNum, targetBuildNum),
			"Otherwise, please input valid build numbers",
			"(example: 13310.1035.0) or valid image names (example: cos-rc-85-13310-1034-0).",
		),
	}
}

// BuildNotFound returns a ChangelogError object for changelog indicating
// the desired build could not be found
func BuildNotFound(buildNumber string) *UtilChangelogError {
	return &UtilChangelogError{
		httpCode: "404",
		header:   "Build Not Found",
		err: strings.Join([]string{
			"The build associated with input",
			buildNumber,
			"cannot be found. It may be possible that the input is either invalid or belongs to a",
			"pre-Cusky build. If you entered a pre-Cusky build number or image name, note that changelog between",
			"pre-Cusky and Cusky builds are not supported. Otherwise, please input a valid build number",
			"(example: 13310.1035.0) or a valid image name (example: cos-rc-85-13310-1034-0).",
		}, " "),
		htmlErr: fmt.Sprintf("%s %s %s<br><br>%s %s %s %s",
			"The build associated with input",
			buildNumber,
			"cannot be found.",
			"It may be possible that either the input is either invalid or belongs to a",
			"pre-Cusky build. If you entered a pre-Cusky build number or image name, note that changelog between",
			"pre-Cusky and Cusky builds are not supported. Otherwise, please input a valid build number",
			"(example: 13310.1035.0) or a valid image name (example: cos-rc-85-13310-1034-0).",
		),
	}
}

func clLink(clID, instanceURL string) string {
	return fmt.Sprintf("<a href=\"%s/c/%s\" target=\"_blank\">CL %s</a>", instanceURL, clID, clID)
}

// CLNotFound returns a ChangelogError object for findbuild indicating the provided
// CL could not be found
func CLNotFound(clID string) *UtilChangelogError {
	return &UtilChangelogError{
		httpCode: "404",
		header:   "CL Not Found",
		err:      fmt.Sprintf("No CL was found matching the identifier: %s. Please enter either the CL-number (example: 3206) or a Commit-SHA (example: I7e549d7753cc7acec2b44bb5a305347a97719ab9) of a submitted CL.", clID),
	}
}

// CLLandingNotFound returns a ChangelogError object for findbuild indicating
// no build was found containing a CL
func CLLandingNotFound(clID, instanceURL string) *UtilChangelogError {
	errStrFmt := "No build was found containing %s."
	link := clLink(clID, instanceURL)
	return &UtilChangelogError{
		httpCode:  "406",
		header:    "No Build Found",
		err:       fmt.Sprintf(errStrFmt, "CL "+clID),
		htmlErr:   fmt.Sprintf(errStrFmt, link),
		retryable: true,
	}
}

// CLNotUsed returns a ChangelogError object for findbuild indicating
// that the repository and branch associated with a CL was not found in any
// manifest files
func CLNotUsed(clID, repo, branch, instanceURL string) *UtilChangelogError {
	errStrFmt := "%s modifies the %s repository on the %s branch, which has not been used in COS builds since the CL's submission."
	link := clLink(clID, instanceURL)
	return &UtilChangelogError{
		httpCode: "406",
		header:   "CL Not Used",
		err:      fmt.Sprintf(errStrFmt, "CL "+clID, repo, branch),
		htmlErr:  fmt.Sprintf(errStrFmt, link, repo, branch),
	}
}

// CLTooRecent returns a ChangelogError object for findbuild indicating the provided
// CL could not be found
func CLTooRecent(clID, instanceURL string) *UtilChangelogError {
	errStrFmt := "%s was submitted too recently to be included in any builds. Please wait a couple hours and try again."
	link := clLink(clID, instanceURL)
	return &UtilChangelogError{
		httpCode: "406",
		header:   "CL Too Recent",
		err:      fmt.Sprintf(errStrFmt, "CL "+clID),
		htmlErr:  fmt.Sprintf(errStrFmt, link),
	}
}

// CLNotSubmitted returns a ChangelogError object for findbuild indicating
// that the provided CL has not been submitted
func CLNotSubmitted(clID, instanceURL string) *UtilChangelogError {
	errStrFmt := "%s has not been submitted yet. A CL will not enter any build until it is successfully submitted."
	link := clLink(clID, instanceURL)
	return &UtilChangelogError{
		httpCode: "406",
		header:   "CL Not Submitted",
		err:      fmt.Sprintf(errStrFmt, "CL "+clID),
		htmlErr:  fmt.Sprintf(errStrFmt, link),
	}
}

// CLInvalidRelease returns a ChangelogError object for findbuild indicating
// that the branch a CL was submitted in was not recognized as a release branch
func CLInvalidRelease(clID, release, instanceURL string) *UtilChangelogError {
	errStrFmt := "%s maps to release %s, which is not a valid release"
	link := clLink(clID, instanceURL)
	return &UtilChangelogError{
		httpCode: "406",
		header:   "Invalid Release Branch",
		err:      fmt.Sprintf(errStrFmt, "CL "+clID, release),
		htmlErr:  fmt.Sprintf(errStrFmt, link, release),
	}
}

// GitilesErrCode parses a Gitiles error message and returns an HTTP error code
// associated with the error. Returns 500 if no error code is found.
func GitilesErrCode(err error) string {
	err = unwrapError(err)
	rpcStatus, ok := status.FromError(err)
	if !ok {
		return "500"
	}
	code, text := rpcStatus.Code(), rpcStatus.Message()
	// RPC status code misclassifies 403 error as 500 error for Gitiles requests
	if code == codes.Internal && text == gitiles403ErrMsg {
		code = codes.PermissionDenied
	}
	if httpCode, ok := grpcCodeToHTTP[code.String()]; ok {
		return httpCode
	}
	return "500"
}

// GerritErrCode parse a Gerrit error and returns an HTTP error code associated
// with the error. Returns 500 if no error code is found.
func GerritErrCode(err error) string {
	err = unwrapError(err)
	matches := gerritErrCodeRe.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return "500"
	}
	return matches[1]
}
