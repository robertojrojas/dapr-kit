/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

import (
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/runtime/protoiface"
)

const (
	Owner   = "components-contrib"
	Domain  = "dapr.io"
	unknown = "UNKNOWN_REASON"
)

const (
	// gRPC to HTTP Mapping: 500 Internal Server Error
	unknownHTTPCode = http.StatusInternalServerError
)

var UnknownErrorReason = WithErrorReason(unknown, unknownHTTPCode, codes.Unknown)

type ResourceInfo struct {
	Type string
	Name string
}

// Option allows passing additional information
// to the Error struct.
// See With* functions for further details.
type Option func(*Error)

// Error encapsulates error information
// with additional details like:
//   - http code
//   - grpcStatus code
//   - error reason
//   - metadata information
//   - optional resourceInfo (componenttype/name)
type Error struct {
	err            error
	description    string
	reason         string
	httpCode       int
	grpcStatusCode codes.Code
	metadata       map[string]string
	resourceInfo   *ResourceInfo
}

// New create a new Error using the supplied metadata and Options
// **Note**: As this code is in `Feature Preview`, it will only continue processing
// if the ErrorCodes is enabled
// TODO: @robertojrojas update when feature is ready.
func New(err error, metadata map[string]string, options ...Option) *Error {
	if err == nil {
		return nil
	}

	// Use default values
	de := &Error{
		err:            err,
		reason:         unknown,
		httpCode:       unknownHTTPCode,
		grpcStatusCode: codes.Unknown,
	}

	// Now apply any requested options
	// to override
	for _, option := range options {
		option(de)
	}

	return de
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e != nil && e.err != nil {
		return e.err.Error()
	}
	return ""
}

// Unwrap implements the error unwrapping interface.
func (e *Error) Unwrap() error {
	return e.err
}

// Description returns the description of the error.
func (e *Error) Description() string {
	if e.description != "" {
		return e.description
	}
	return e.err.Error()
}

// WithErrorReason used to pass reason, httpCode, and
// grpcStatus code to the Error struct.
func WithErrorReason(reason string, httpCode int, grpcStatusCode codes.Code) Option {
	return func(err *Error) {
		err.reason = reason
		err.grpcStatusCode = grpcStatusCode
		err.httpCode = httpCode
	}
}

// WithResourceInfo used to pass ResourceInfo to the Error struct.
func WithResourceInfo(resourceInfo *ResourceInfo) Option {
	return func(e *Error) {
		e.resourceInfo = resourceInfo
	}
}

// WithDescription used to pass a description
// to the Error struct.
func WithDescription(description string) Option {
	return func(e *Error) {
		e.description = description
	}
}

// WithMetadata used to pass a Metadata[string]string
// to the Error struct.
func WithMetadata(md map[string]string) Option {
	return func(e *Error) {
		e.metadata = md
	}
}

func newErrorInfo(reason string, md map[string]string) *errdetails.ErrorInfo {
	return &errdetails.ErrorInfo{
		Domain:   Domain,
		Reason:   reason,
		Metadata: md,
	}
}

func newResourceInfo(rid *ResourceInfo, err error) *errdetails.ResourceInfo {
	return &errdetails.ResourceInfo{
		ResourceType: rid.Type,
		ResourceName: rid.Name,
		Owner:        Owner,
		Description:  err.Error(),
	}
}

// *** GRPC Methods ***

// GRPCStatus returns the gRPC status.Status object.
func (e *Error) GRPCStatus() *status.Status {
	messages := []protoiface.MessageV1{
		newErrorInfo(e.reason, e.metadata),
	}

	if e.resourceInfo != nil {
		messages = append(messages, newResourceInfo(e.resourceInfo, e.err))
	}

	ste, stErr := status.New(e.grpcStatusCode, e.description).WithDetails(messages...)
	if stErr != nil {
		return status.New(e.grpcStatusCode, e.description)
	}

	return ste
}

// *** HTTP Methods ***

// ToHTTP transforms the supplied error into
// a GRPC Status and then Marshals it to JSON.
// It assumes if the supplied error is of type Error.
// Otherwise, returns the original error.
func (e *Error) ToHTTP() (int, []byte) {
	resp, err := protojson.Marshal(e.GRPCStatus().Proto())
	if err != nil {
		return http.StatusInternalServerError, []byte(err.Error())
	}

	return e.httpCode, resp
}

// HTTPCode returns the value of the HTTPCode property.
func (e *Error) HTTPCode() int {
	if e.httpCode == 0 {
		return http.StatusInternalServerError
	}
	return e.httpCode
}

// JSONErrorValue implements the errorResponseValue interface.
func (e *Error) JSONErrorValue() []byte {
	b, err := protojson.Marshal(e.GRPCStatus().Proto())
	if err != nil {
		return []byte(err.Error())
	}
	return b
}
