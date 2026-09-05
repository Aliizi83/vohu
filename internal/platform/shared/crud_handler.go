package shared

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// The five handlers below mirror sample-golang-project's
// api/handlers/base_generic_crud.go (Create/Update/Delete/GetById/
// GetByFilter), rebuilt on this project's BaseResponse instead of their
// helpers package. mapErr is supplied per call site (rather than a global
// error->status table) so this package never has to import a module's
// sentinel errors just to know how to respond to them.

type ErrorMapper func(error) (status int, code ResultCode)

func CreateHandler[TRequest, TInput, TOutput, TResponse any](
	c *gin.Context,
	mapReq func(TRequest) TInput,
	mapRes func(TOutput) TResponse,
	create func(ctx context.Context, input TInput) (TOutput, error),
	mapErr ErrorMapper,
) {
	var req TRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	output, err := create(c.Request.Context(), mapReq(req))
	if err != nil {
		status, code := mapErr(err)
		RespondError(c, status, code, err)
		return
	}

	RespondSuccess(c, http.StatusCreated, mapRes(output))
}

func UpdateHandler[TRequest, TInput, TOutput, TResponse any](
	c *gin.Context,
	mapReq func(TRequest) TInput,
	mapRes func(TOutput) TResponse,
	update func(ctx context.Context, id uint, input TInput) (TOutput, error),
	mapErr ErrorMapper,
) {
	id, err := ParseIDParam(c)
	if err != nil {
		RespondError(c, http.StatusBadRequest, ResultValidationError, errors.New("invalid id"))
		return
	}

	var req TRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	output, err := update(c.Request.Context(), id, mapReq(req))
	if err != nil {
		status, code := mapErr(err)
		RespondError(c, status, code, err)
		return
	}

	RespondSuccess(c, http.StatusOK, mapRes(output))
}

func GetByIDHandler[TOutput, TResponse any](
	c *gin.Context,
	mapRes func(TOutput) TResponse,
	getByID func(ctx context.Context, id uint) (TOutput, error),
	mapErr ErrorMapper,
) {
	id, err := ParseIDParam(c)
	if err != nil {
		RespondError(c, http.StatusBadRequest, ResultValidationError, errors.New("invalid id"))
		return
	}

	output, err := getByID(c.Request.Context(), id)
	if err != nil {
		status, code := mapErr(err)
		RespondError(c, status, code, err)
		return
	}

	RespondSuccess(c, http.StatusOK, mapRes(output))
}

func DeleteHandler(
	c *gin.Context,
	del func(ctx context.Context, id uint) error,
	mapErr ErrorMapper,
) {
	id, err := ParseIDParam(c)
	if err != nil {
		RespondError(c, http.StatusBadRequest, ResultValidationError, errors.New("invalid id"))
		return
	}

	if err := del(c.Request.Context(), id); err != nil {
		status, code := mapErr(err)
		RespondError(c, status, code, err)
		return
	}

	RespondSuccess(c, http.StatusOK, nil)
}

// ListRequest is the request body GetByFilter-style list endpoints bind:
// pagination plus the dynamic filter, same shape as
// sample-golang-project's PaginationInputWithFilter.
type ListRequest struct {
	Pagination
	DynamicFilter
}

func ListHandler[TOutput, TResponse any](
	c *gin.Context,
	mapRes func(TOutput) TResponse,
	list func(ctx context.Context, filter DynamicFilter, page Pagination) ([]TOutput, int64, error),
) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondValidationError(c, err)
		return
	}

	items, total, err := list(c.Request.Context(), req.DynamicFilter, req.Pagination)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
		return
	}

	responses := make([]TResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, mapRes(item))
	}

	RespondSuccess(c, http.StatusOK, NewPagedList(responses, total, req.Pagination))
}

// Identity is a trivial mapper for CreateHandler/UpdateHandler's
// requestMapper/responseMapper params when a module's API-level DTO
// already *is* what the service takes/returns — no separate api-dto vs
// service-dto split like sample-golang-project has.
func Identity[T any](v T) T {
	return v
}
