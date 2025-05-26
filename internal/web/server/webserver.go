package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/monopole/mdrip/v2/internal/shell"
	"github.com/monopole/mdrip/v2/internal/utils"
	"github.com/monopole/mdrip/v2/internal/web/config"
	"github.com/monopole/mdrip/v2/internal/web/server/minify"
)

const (
	cookieName = utils.PgmName
)

var (
	//  keyAuth = securecookie.GenerateRandomKey(16)
	keyAuth    = []byte("static-visible-secret-who-cares")
	keyEncrypt = []byte(nil)
)

// Server represents a webserver.
type Server struct {
	// dLoader loads markdown to serve.
	dLoader *DataLoader
	// minifier minifies generates html, js and css before serving it.
	minifier *minify.Minifier
	// store manages cookie state - experimental, not sure that
	// it's useful to store app state.  FWIW, it attempts to put you on the same
	// codeblock if you reload (start a new session).
	store sessions.Store
	// codeWriter accepts codeblocks for execution or simply printing.
	codeWriter io.Writer
	// managedShell is a controllable shell for executing commands.
	managedShell *shell.ManagedShell
}

// NewServer returns a new web server.
func NewServer(dl *DataLoader, r io.Writer) (*Server, error) {
	s := sessions.NewCookieStore(keyAuth, keyEncrypt)
	s.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   8 * 60 * 60, // 8 hours (Max-Age has units seconds)
		HttpOnly: true,
	}
	// Initialize ManagedShell
	// For now, using /bin/bash. This could be configurable.
	ms, err := shell.NewManagedShell("/bin/bash")
	if err != nil {
		slog.Error("Failed to create new managed shell", "err", err)
		return nil, err
	}
	if err := ms.Start(); err != nil {
		slog.Error("Failed to start managed shell", "err", err)
		// Consider if we should attempt to Stop/cleanup ms here, though Start failing might mean it's not fully initialized.
		return nil, err
	}
	slog.Info("Managed shell started successfully.")

	// TODO: Decide on the fate of codeWriter. For now, it's kept.
	// If r (io.Writer for codeWriter) was specifically for a previous mechanism like tmux,
	// and ManagedShell is the new primary, we might deprecate/remove codeWriter.
	// For now, let's log if 'r' is not nil, as it might indicate an old usage pattern.
	if r != nil {
		slog.Warn("codeWriter (io.Writer) is still being passed to NewServer. " +
			"Ensure this is intended, as ManagedShell is now the primary for code execution.")
	}

	return &Server{
		dLoader:      dl,
		store:        s,
		minifier:     minify.MakeMinifier(),
		codeWriter:   r, // Kept for now
		managedShell: ms,
	}, nil
}

// AppConfig holds configuration to be passed to the frontend.
type AppConfig struct {
	RunBlockURL string `json:"runBlockURL"`
}

// Serve offers an HTTP service.
func (ws *Server) Serve(hostAndPort string) (err error) {
	http.HandleFunc("/favicon.ico", ws.handleFavicon)
	http.HandleFunc("/static/", ws.handleStaticFiles) // Ensure this is before the catch-all
	http.HandleFunc(config.Dynamic(config.RouteLissajous), ws.handleLissajous)
	http.HandleFunc(config.Dynamic(config.RouteQuit), ws.handleQuit)
	http.HandleFunc(config.Dynamic(config.RouteDebug), ws.handleDebugPage)
	http.HandleFunc(config.Dynamic(config.RouteReload), ws.handleReload)
	// http.Handle(session.Dynamic(session.RouteWebSocket), ws.openWebSocket)
	http.HandleFunc(config.Dynamic(config.RouteJs), ws.handleGetJs)
	http.HandleFunc(config.Dynamic(config.RouteCss), ws.handleGetCss)
	http.HandleFunc(config.Dynamic(config.RouteLabelsForFile), ws.handleGetLabelsForFile)
	http.HandleFunc(config.Dynamic(config.RouteHtmlForFile), ws.handleGetHtmlForFile)
	http.HandleFunc(config.Dynamic(config.RouteRunBlock), ws.handleRunCodeBlock)
	http.HandleFunc(config.Dynamic(config.RouteSave), ws.handleSaveSession)

	// In server mode, the dLoader.paths slice has exactly one entry,
	// so we only need the [0] entry and we know it is there.
	dir := strings.TrimSuffix(ws.dLoader.paths[0], "/")
	slog.Info("Serving static content from ", "dir", dir)
	http.Handle("/", ws.makeMetaHandler(http.FileServer(http.Dir(dir))))

	slog.Info("Serving at " + hostAndPort)
	if err = http.ListenAndServe(hostAndPort, nil); err != nil {
		slog.Error("unable to start server", "err", err)
	}
	return err
}

func (ws *Server) makeMetaHandler(fsHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		slog.Info("got request for", "url", req.URL)
		if strings.HasSuffix(req.URL.Path, "/") ||
			// trigger markdown rendering
			strings.HasSuffix(req.URL.Path, ".md") {
			ws.handleRenderWebApp(w, req)
			return
		}
		// just serve a file.
		fsHandler.ServeHTTP(w, req)
	})
}

func (ws *Server) handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	// This assumes 'internal/web/static' is the root for these static files,
	// relative to the directory from which the application is run.
	// More robust would be to use an embedded FS or path relative to executable.
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir("internal/web/static")))
	fs.ServeHTTP(w, r)
}

// ExecResponse is the structure for JSON responses from code execution.
type ExecResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"error,omitempty"`
}

func (ws *Server) handleRunCodeBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	if ws.managedShell == nil {
		slog.Error("Managed shell is not initialized.")
		http.Error(w, "Internal server error: shell not available", http.StatusInternalServerError)
		return
	}

	codeBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body", "err", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	command := string(codeBytes)
	slog.Info("Executing command in managed shell", "command", command)

	stdout, stderr, execErr := ws.managedShell.Execute(command)

	response := ExecResponse{
		Stdout: stdout,
		Stderr: stderr,
	}

	if execErr != nil {
		slog.Error("Error executing command in managed shell", "err", execErr, "stdout", stdout, "stderr", stderr)
		response.Error = execErr.Error()
		// Decide if we should send a 500 or just include error in JSON
		// For now, we'll send 200 with error in JSON, as the execution attempt was made.
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("Failed to marshal JSON response", "err", err)
		http.Error(w, "Error creating response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResponse); err != nil {
		slog.Error("Failed to write JSON response", "err", err)
		// Client response already started, so can't send a different HTTP error code.
	}
}
