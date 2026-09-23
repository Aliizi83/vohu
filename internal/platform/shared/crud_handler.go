package shared

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
	id, ok := RequireIDParam(c)
	if !ok {
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
	id, ok := RequireIDParam(c)
	if !ok {
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
	id, ok := RequireIDParam(c)
	if !ok {
		return
	}

	if err := del(c.Request.Context(), id); err != nil {
		status, code := mapErr(err)
		RespondError(c, status, code, err)
		return
	}

	RespondSuccess(c, http.StatusOK, nil)
}

type listQuery struct {
	PageNumber int    `form:"pageNumber"`
	PageSize   int    `form:"pageSize"`
	Filter     string `form:"filter"`
}

func ParseListQuery(c *gin.Context) (Pagination, DynamicFilter, error) {
	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		return Pagination{}, DynamicFilter{}, err
	}

	var filter DynamicFilter
	if q.Filter != "" {
		if err := json.Unmarshal([]byte(q.Filter), &filter); err != nil {
			return Pagination{}, DynamicFilter{}, errors.New("invalid filter")
		}
	}

	return Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}, filter, nil
}

func ListHandler[TOutput, TResponse any](
	c *gin.Context,
	mapRes func(TOutput) TResponse,
	list func(ctx context.Context, filter DynamicFilter, page Pagination) ([]TOutput, int64, error),
) {
	page, filter, err := ParseListQuery(c)
	if err != nil {
		RespondValidationError(c, err)
		return
	}

	items, total, err := list(c.Request.Context(), filter, page)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
		return
	}

	responses := make([]TResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, mapRes(item))
	}

	RespondSuccess(c, http.StatusOK, NewPagedList(responses, total, page))
}

func Identity[T any](v T) T {
	return v
}
