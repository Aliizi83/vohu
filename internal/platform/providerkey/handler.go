package providerkey

import (
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func mapError(err error) (int, shared.ResultCode) {
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, shared.ResultNotFoundError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

// SetGlobal sets the account-wide default key for a provider.
//
//	@Summary		Set the global provider key
//	@Description	Sets (or replaces) the account-wide default API key for a provider — used for any user who hasn't set their own personal key. The key is encrypted at rest and never returned by any response. Requires wildcard "manage" access on resource type "provider_key".
//	@Tags			provider-keys
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SetKeyRequest	true	"Provider key"
//	@Success		200		{object}	shared.BaseResponse
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys [post]
func (h *Handler) SetGlobal(c *gin.Context) {
	var req SetKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.SetGlobal(c.Request.Context(), req); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

// ListGlobal lists every global provider key.
//
//	@Summary		List global provider keys
//	@Description	Lists every account-wide default provider key (never the key value itself). Requires wildcard "manage" access on resource type "provider_key".
//	@Tags			provider-keys
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=[]Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys [get]
func (h *Handler) ListGlobal(c *gin.Context) {
	items, err := h.service.ListGlobal(c.Request.Context())
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

// DeleteGlobal removes the global key for a provider.
//
//	@Summary		Remove the global provider key
//	@Description	Removes the account-wide default key for a provider. Chats using it fall back to any other configured key. Requires wildcard "manage" access on resource type "provider_key".
//	@Tags			provider-keys
//	@Produce		json
//	@Param			provider	path		string	true	"Provider"	Enums(gemini, anthropic, openai)
//	@Success		200			{object}	shared.BaseResponse
//	@Failure		401			{object}	shared.BaseResponse
//	@Failure		404			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys/{provider} [delete]
func (h *Handler) DeleteGlobal(c *gin.Context) {
	provider := Provider(c.Param("provider"))
	if err := h.service.DeleteGlobal(c.Request.Context(), provider); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}

// SetMine sets the caller's own personal key for a provider.
//
//	@Summary		Set my provider key
//	@Description	Sets (or replaces) the caller's own personal API key for a provider — always takes priority over the global default for that caller's own chats. The key is encrypted at rest and never returned by any response.
//	@Tags			provider-keys
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SetKeyRequest	true	"Provider key"
//	@Success		200		{object}	shared.BaseResponse
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys/me [post]
func (h *Handler) SetMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req SetKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.SetMine(c.Request.Context(), userID, req); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

// ListMine lists the caller's own provider keys.
//
//	@Summary		List my provider keys
//	@Description	Lists the caller's own personal provider keys (never the key values themselves).
//	@Tags			provider-keys
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=[]Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys/me [get]
func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	items, err := h.service.ListMine(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

// DeleteMine removes the caller's own key for a provider.
//
//	@Summary		Remove my provider key
//	@Description	Removes the caller's own personal key for a provider. Their chats fall back to the global default, if any.
//	@Tags			provider-keys
//	@Produce		json
//	@Param			provider	path		string	true	"Provider"	Enums(gemini, anthropic, openai)
//	@Success		200			{object}	shared.BaseResponse
//	@Failure		401			{object}	shared.BaseResponse
//	@Failure		404			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/provider-keys/me/{provider} [delete]
func (h *Handler) DeleteMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	provider := Provider(c.Param("provider"))
	if err := h.service.DeleteMine(c.Request.Context(), userID, provider); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}
