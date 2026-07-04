package http

import (
	"maps"
	"math"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// defaultErrorStatus is the HTTP status used for an error envelope built
// directly with RES when no explicit Status is set. HTTP status is a
// transport concern owned by the route wrappers (see Router); a bare RES
// error is treated as a soft, in-body failure and stays HTTP 200.
const defaultErrorStatus = fiber.StatusOK

const (
	statusSuccess = "success"
	statusError   = "error"
)

// ErrorDetail is the JSON wire shape of a single API error entry.
type ErrorDetail struct {
	Kind    string `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	Cause   string `json:"cause,omitempty"`
}

// ErrorDetailFrom maps a canonical error to its API wire shape.
func ErrorDetailFrom(err *errors.Error) ErrorDetail {
	if err == nil {
		return ErrorDetail{}
	}
	detail := ErrorDetail{
		Kind:    err.Kind,
		Code:    err.Code,
		Message: err.Message,
		Source:  err.Source,
	}
	if cause := errors.Cause(err); cause != nil {
		detail.Cause = cause.Error()
	}
	return detail
}

// Response builds a JSON API envelope. Success responses use HTTP 200; error
// responses stay HTTP 200 unless an explicit Status is set. Route wrappers
// (Router) resolve semantic status from returned catalog errors.
type Response struct {
	ctx fiber.Ctx

	requestID       string
	clientRequestID string
	status          string
	message         string
	httpStatus      int

	data       map[string]any
	errs       []ErrorDetail
	pagination *Pagination
	cursor     *Cursor
}

// Pagination describes page-based list metadata.
type Pagination struct {
	TotalCount int  `json:"total_count,omitempty"`
	TotalPages int  `json:"total_pages,omitempty"`
	MaxPage    int  `json:"max_page,omitempty"`
	Page       int  `json:"page,omitempty"`
	Size       int  `json:"size,omitempty"`
	First      bool `json:"first"`
	Last       bool `json:"last"`
}

// Cursor describes cursor-based list metadata.
type Cursor struct {
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

type envelope struct {
	Status          string         `json:"status"`
	Message         string         `json:"message,omitempty"`
	RequestID       string         `json:"request_id,omitempty"`
	ClientRequestID string         `json:"client_request_id,omitempty"`
	Data            map[string]any `json:"data,omitempty"`
	Cursor          *Cursor        `json:"cursor,omitempty"`
	Pagination      *Pagination    `json:"pagination,omitempty"`
	Errors          []ErrorDetail  `json:"errors,omitempty"`
}

// RES starts building a response for a Fiber request.
func RES(c fiber.Ctx) *Response {
	return &Response{
		ctx:             c,
		requestID:       requestid.FromContext(c),
		clientRequestID: ClientRequestIDFromContext(c),
		status:          statusSuccess,
	}
}

// Message sets the top-level response message.
func (r *Response) Message(msg string) *Response {
	r.message = msg
	return r
}

// Status sets an explicit HTTP status for the response. It overrides the
// HTTP-200 default for error envelopes and is what the Router uses to apply a
// resolved status.
func (r *Response) Status(code int) *Response {
	r.httpStatus = code
	return r
}

// Data adds a keyed value to the success payload.
func (r *Response) Data(key string, value any) *Response {
	if r.data == nil {
		r.data = map[string]any{}
	}
	r.data[key] = value
	return r
}

// MergeData merges values into the success payload.
func (r *Response) MergeData(values map[string]any) *Response {
	if len(values) == 0 {
		return r
	}
	if r.data == nil {
		r.data = map[string]any{}
	}
	maps.Copy(r.data, values)
	return r
}

// Paginator attaches page pagination metadata.
func (r *Response) Paginator(page, size, offset, total int) *Response {
	totalPages := float64(total) / float64(size)
	if math.Mod(totalPages, 1.0) > 0 {
		totalPages++
	}

	pagination := &Pagination{
		TotalCount: total,
		TotalPages: int(totalPages),
		MaxPage:    1,
		Page:       page,
		Size:       size,
	}

	if pagination.TotalPages < 1 {
		pagination.TotalPages = 0
	}
	if pagination.TotalPages > 0 {
		pagination.MaxPage = pagination.TotalPages
	}

	pagination.First = offset < 1
	pagination.Last = pagination.MaxPage == pagination.Page

	r.pagination = pagination
	return r
}

// Cursor attaches cursor pagination metadata.
func (r *Response) Cursor(next, previous string) *Response {
	r.cursor = &Cursor{Next: next, Previous: previous}
	return r
}

// Error appends an API error and marks the response as failed.
func (r *Response) Error(err *errors.Error) *Response {
	if err == nil {
		return r
	}
	r.status = statusError
	r.errs = append(r.errs, ErrorDetailFrom(err))
	return r
}

// Errors appends multiple API errors and marks the response as failed.
func (r *Response) Errors(errs ...*errors.Error) *Response {
	for _, err := range errs {
		r.Error(err)
	}
	return r
}

// Apply appends API errors extracted from err.
func (r *Response) Apply(err error) *Response {
	return r.ApplyErrors(errors.AsErrors(err))
}

// ApplyErrors appends a collection of canonical errors.
func (r *Response) ApplyErrors(errs errors.Errors) *Response {
	for _, err := range errs {
		r.Error(err)
	}
	return r
}

// IsError reports whether the response has error status.
func (r *Response) IsError() bool {
	return r.status == statusError
}

// Ok writes the JSON envelope. Success responses use HTTP 200. Error responses
// use the explicit Status when set, otherwise stay HTTP 200.
func (r *Response) Ok() error {
	out := envelope{
		Status:          r.status,
		Message:         r.message,
		RequestID:       r.requestID,
		ClientRequestID: r.clientRequestID,
	}

	if r.status == statusSuccess {
		if len(r.data) > 0 {
			out.Data = r.data
		}
		if r.cursor != nil {
			out.Cursor = r.cursor
		}
		if r.pagination != nil {
			out.Pagination = r.pagination
		}
	} else if len(r.errs) > 0 {
		out.Errors = r.errs
	}

	status := fiber.StatusOK
	if r.status == statusError {
		status = defaultErrorStatus
		if r.httpStatus > 0 {
			status = r.httpStatus
		}
	}

	return r.ctx.Status(status).JSON(out)
}
