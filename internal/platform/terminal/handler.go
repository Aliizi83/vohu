package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/pkg/logging"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type Handler struct {
	tickets  *TicketStore
	sshconns sshconn.Service
	logger   logging.Logger
}

func NewHandler(tickets *TicketStore, sshconns sshconn.Service, logger logging.Logger) *Handler {
	return &Handler{tickets: tickets, sshconns: sshconns, logger: logger}
}

// IssueTicket mints a one-time WebSocket ticket for a connection the
// caller has already been confirmed to hold "write" access to (see
// routes.go) — the ticket is the only credential ServeWS ever sees.
//
//	@Summary		Get a one-time ticket to open a web terminal
//	@Description	Mints a short-lived (30s), single-use ticket for GET /ssh-connections/{id}/terminal-ws — needed because a WebSocket handshake can't carry the normal Bearer header. Requires "write" access to this connection.
//	@Tags			ssh-connections
//	@Produce		json
//	@Param			id	path		int	true	"SSH connection ID"
//	@Success		200	{object}	shared.BaseResponse{result=object{ticket=string}}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections/{id}/terminal-ticket [post]
func (h *Handler) IssueTicket(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	connectionID, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	ticket, err := h.tickets.Issue(c.Request.Context(), userID, connectionID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, gin.H{"ticket": ticket})
}

var upgrader = websocket.Upgrader{
	// The frontend always talks to this same origin (proxied in dev,
	// same-origin in prod) — nothing here ever needs a cross-origin
	// WebSocket, so there's no CORS allow-list to maintain.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS is unauthenticated by the normal Bearer middleware (see
// routes.go) — a valid, unexpired, single-use ticket naming this exact
// connection is the only thing that gets a caller past this point.
//
//	@Summary		Open a web terminal (WebSocket)
//	@Description	Upgrades to a WebSocket and bridges it to an interactive SSH shell (a real PTY, not run through any command allow-list — see internal/tools/command.Policy's doc comment for why that's a deliberate, separate trust boundary from ssh_execute). Client→server messages are JSON: {"type":"input","data":"..."} for keystrokes, {"type":"resize","cols":N,"rows":N} for terminal resizes. Server→client messages are the shell's raw output. ticket comes from POST .../terminal-ticket.
//	@Tags			ssh-connections
//	@Param			id		path	int		true	"SSH connection ID"
//	@Param			ticket	query	string	true	"One-time ticket from POST .../terminal-ticket"
//	@Success		101		{string}	string	"Switching Protocols"
//	@Failure		400		{object}	shared.BaseResponse
//	@Router			/ssh-connections/{id}/terminal-ws [get]
func (h *Handler) ServeWS(c *gin.Context) {
	connectionID, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	ticket := c.Query("ticket")
	_, ticketConnectionID, err := h.tickets.Redeem(c.Request.Context(), ticket)
	if err != nil || ticketConnectionID != connectionID {
		shared.RespondError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("invalid or expired ticket"))
		return
	}

	conn, err := h.sshconns.GetByID(c.Request.Context(), connectionID)
	if err != nil {
		shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
		return
	}
	privateKey, err := h.sshconns.DecryptPrivateKey(conn)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", conn.Host, conn.Port), &ssh.ClientConfig{
		User:            conn.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("failed to connect: "+err.Error()+"\r\n"))
		return
	}
	defer client.Close()

	if err := bridge(ws, client); err != nil {
		h.logger.Error(err, logging.General, logging.Terminal, "terminal session ended with an error", nil)
	}
}

type clientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// bridge pumps bytes between the WebSocket and an interactive SSH shell
// until either side closes — session.Wait() only returns once the remote
// shell itself exits, so the read loop below is what notices the browser
// side going away and tears the SSH session down in response.
func bridge(ws *websocket.Conn, client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	outR, outW := io.Pipe()
	session.Stdout = outW
	session.Stderr = outW

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		return err
	}
	if err := session.Shell(); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := outR.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			break
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			stdin.Write([]byte(msg.Data))
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				session.WindowChange(msg.Rows, msg.Cols)
			}
		}
	}

	outW.Close()
	return session.Wait()
}
