package server

import (
	"encoding/json"
	"fmt"
	htmlTmpl "html/template"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/monopole/mdrip/v2/internal/loader"
	"github.com/monopole/mdrip/v2/internal/web/app"
	"github.com/monopole/mdrip/v2/internal/web/app/widget/common"
	"github.com/monopole/mdrip/v2/internal/web/app/widget/mdrip"
	"github.com/monopole/mdrip/v2/internal/web/app/widget/session"
	"github.com/monopole/mdrip/v2/internal/web/config"
	"github.com/monopole/mdrip/v2/internal/web/server/minify"
)

// handleRenderWebApp sends a full "single-page" web app.
// The app does XHRs as you click around or use keys.
func (ws *Server) handleRenderWebApp(wr http.ResponseWriter, req *http.Request) {
	slog.Info("Rendering web app", "req", req.URL)
	var err error
	mySess, _ := ws.store.Get(req, cookieName)
	session.AssureDefaults(mySess)
	if err = mySess.Save(req, wr); err != nil {
		write500(wr, fmt.Errorf("session save fail; %w", err))
		return
	}
	if err = ws.dLoader.LoadAndRender(); err != nil {
		write500(wr, fmt.Errorf("data loader fail; %w", err))
		return
	}
	var tmpl *htmlTmpl.Template
	tmpl, err = common.ParseAsHtmlTemplate(app.AsTmpl())
	if err != nil {
		write500(wr, fmt.Errorf("template parsing fail; %w", err))
		return
	}
	ws.dLoader.appState.SetInitialFileIndex(req.URL.Path)

	// Prepare AppConfig
	appCfg := AppConfig{ // Assumes AppConfig is defined in the same package (e.g. in webserver.go)
		RunBlockURL: config.Dynamic(config.RouteRunBlock),
	}
	appConfigBytes, err := json.Marshal(appCfg)
	if err != nil {
		slog.Error("Failed to marshal AppConfig", "err", err)
		write500(wr, fmt.Errorf("failed to marshal app config: %w", err))
		return
	}
	appConfigJSONString := string(appConfigBytes)

	// Original params (we might not need all of them if AppState is passed directly)
	// originalTemplateParams := mdrip.MakeParams(ws.dLoader.navLeftRoot, ws.dLoader.appState)

	// Create a map for template data to ensure AppState and AppConfigJSON are at the root
	templateData := map[string]interface{}{
		// AppState is used by the template as {{.AppState...}}
		"AppState": ws.dLoader.appState,
		// AppConfigJSON is used by the template as {{.AppConfigJSON}}
		"AppConfigJSON": appConfigJSONString,
		// Add other parameters from originalTemplateParams if they are directly accessed
		// at the root level in the template and are not part of AppState.
		// For example, if originalTemplateParams had a field like "PageTitle"
		// and template used {{.PageTitle}}, we'd need to add it here.
		// For now, we assume AppState and AppConfigJSON are the primary top-level needs
		// based on the webapp.go template structure.
		// The call to mdrip.MakeParams might do more than just return AppState,
		// e.g., it might set specific fields on AppState or return a struct containing AppState.
		// If mdrip.MakeParams returns a struct that *is* AppState or contains all necessary fields including AppState,
		// then this approach is fine.
		// If mdrip.MakeParams returns a struct, and the template needs other fields from this struct
		// (e.g. {{.NavLeftRootData}}), this map approach would need to explicitly include them.
		// Given the constraints, providing AppState directly is the most robust way to satisfy {{.AppState...}}
		// and then adding AppConfigJSON alongside it.
	}

	err = tmpl.ExecuteTemplate(wr, app.TmplName, templateData)
	if err != nil {
		write500(wr, fmt.Errorf("template rendering failure; %w", err))
		return
	}
}

func (ws *Server) handleSaveSession(w http.ResponseWriter, r *http.Request) {
	slog.Info("Saving session", "req", r.URL)
	s, err := ws.store.Get(r, cookieName)
	if err != nil {
		write500(w, err)
		return
	}
	s.Values[config.KeyIsNavOn] = getBoolParam(config.KeyIsNavOn, r, false)
	s.Values[config.KeyIsTitleOn] = getBoolParam(config.KeyIsTitleOn, r, false)
	s.Values[config.KeyMdFileIndex] = getIntParam(config.KeyMdFileIndex, r, 0)
	s.Values[config.KeyBlockIndex] = getIntParam(config.KeyBlockIndex, r, 0)
	if err = s.Save(r, w); err != nil {
		slog.Error("unable to save session", "err", err)
	}
	_, _ = fmt.Fprintln(w, "Ok")
	slog.Info("Saved session.")
}

func (ws *Server) handleGetHtmlForFile(wr http.ResponseWriter, req *http.Request) {
	slog.Info("handleGetHtmlForFile ", "req", req.URL)
	f, err := ws.getRenderedMdFile(req)
	if err != nil {
		write500(wr, fmt.Errorf("handleGetHtmlForFile render; %w", err))
		return
	}
	_, err = wr.Write([]byte(f.Html))
	if err != nil {
		write500(wr, fmt.Errorf("handleGetHtmlForFile write; %w", err))
		return
	}
	slog.Info("handleGetHtmlForFile success")
}

func (ws *Server) handleGetLabelsForFile(wr http.ResponseWriter, req *http.Request) {
	slog.Info("handleGetLabelsForFile ", "req", req.URL)
	f, err := ws.getRenderedMdFile(req)
	if err != nil {
		write500(wr, fmt.Errorf("handleGetLabelsForFile render; %w", err))
		return
	}
	var jsn []byte
	jsn, err = json.Marshal(loader.NewLabelList(f.Blocks))
	if err != nil {
		write500(wr, fmt.Errorf("handleGetLabelsForFile marshal; %w", err))
		return
	}
	if _, err = wr.Write(jsn); err != nil {
		write500(wr, fmt.Errorf("handleGetLabelsForFile write; %w", err))
		return
	}
	slog.Info("handleGetLabelsForFile success")
}

func (ws *Server) handleGetJs(wr http.ResponseWriter, req *http.Request) {
	slog.Info("handleGetJs", "req", req.URL)
	ws.minifier.Write(wr, &minify.Args{
		MimeType: app.MimeJs,
		Tmpl: minify.TmplArgs{
			Name: mdrip.TmplNameJs,
			Body: mdrip.AsTmplJs(),
			Params: mdrip.MakeBaseParams(
				ws.dLoader.appState.Facts.MaxNavWordLength),
		},
	})
}

func (ws *Server) handleGetCss(wr http.ResponseWriter, req *http.Request) {
	slog.Info("handleGetCss", "req", req.URL)
	ws.minifier.Write(wr, &minify.Args{
		MimeType: app.MimeCss,
		Tmpl: minify.TmplArgs{
			Name: mdrip.TmplNameCss,
			Body: mdrip.AsTmplCss(),
			Params: mdrip.MakeBaseParams(
				ws.dLoader.appState.Facts.MaxNavWordLength),
		},
	})
}
func (ws *Server) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	Lissajous(w, 7, 3, 1)
}

func (ws *Server) handleLissajous(w http.ResponseWriter, r *http.Request) {
	mySess, _ := ws.store.Get(r, cookieName)
	_ = mySess.Save(r, w)
	Lissajous(w,
		getIntParam("s", r, 300),
		getIntParam("c", r, 30),
		getIntParam("n", r, 100))
}

// handleReload forces a data reload.
func (ws *Server) handleReload(wr http.ResponseWriter, req *http.Request) {
	slog.Info("Handling data reload", "url", req.URL)
	if err := ws.reload(wr, req); err != nil {
		write500(wr, fmt.Errorf("handleReload; %w", err))
		return
	}
}

// handleDebugPage forces a data reload and shows a debug page.
func (ws *Server) handleDebugPage(wr http.ResponseWriter, req *http.Request) {
	slog.Info("Rendering debug page", "url", req.URL)
	if err := ws.reload(wr, req); err != nil {
		write500(wr, fmt.Errorf("handleDebugPage; %w", err))
		return
	}
	ws.dLoader.folder.Accept(loader.NewVisitorDump(wr))
	loader.DumpBlocks(wr, ws.dLoader.FilteredBlocks())
}

func (ws *Server) handleQuit(w http.ResponseWriter, _ *http.Request) {
	slog.Info("Received quit.")
	_, _ = fmt.Fprint(w, "\nbye bye\n")
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}
