package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stalane/toolcheck/internal/check"
	"github.com/stalane/toolcheck/internal/config"
	"github.com/stalane/toolcheck/internal/present"
	"github.com/stalane/toolcheck/internal/registry"
	"github.com/stalane/toolcheck/internal/store"
	"github.com/stalane/toolcheck/internal/update"
)

type Server struct {
	store   *store.Store
	addr    string
	cfg     config.Config
	engine  *check.Engine
	updater *update.Updater
	log     *slog.Logger
	csrf    string
	tmpl    *template.Template
	static  http.Handler
}

type toolView struct {
	ID              int64
	Name            string
	Family          string
	Version         string
	Latest          string
	Origin          string
	ManagedBy       string
	Risk            string
	Path            string
	RealPath        string
	Hint            string
	State           string
	Published       string
	Age             string
	UpdateAvailable bool
	Updateable      bool
	System          bool
	Active          bool
	Security        bool
}

type dashboardData struct {
	Tools        []toolView
	UpdateCount  int
	AlertCount   int
	ManagedCount int
	Total        int
	LastCheck    string
	Addr         string
	CSRF         string
	Message      string
	Error        string
}

type toolData struct {
	Tool    toolView
	CSRF    string
	Updates []updateView
	Message string
	Error   string
}

type checkView struct {
	Started  string
	Finished string
	Status   string
	Tools    int
	Updates  int
	Error    string
}

type updateView struct {
	Tool     string
	From     string
	To       string
	Status   string
	Command  string
	Output   string
	Started  string
	Finished string
}

type activityData struct {
	Checks  []checkView
	Updates []updateView
}

type settingsData struct {
	ConfigPath    string
	DataDir       string
	DBPath        string
	Addr          string
	LastCheck     string
	Interval      string
	CacheTTL      string
	NodeChannel   string
	MinReleaseAge string
	UpdatesOn     bool
	CSRF          string
}

func New(st *store.Store, addr string, cfg config.Config, engine *check.Engine, updater *update.Updater, log *slog.Logger) *Server {
	tmpl := template.Must(template.ParseFS(assets, "templates/*.html"))
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	if updater == nil {
		updater = update.New(st)
	}
	return &Server{
		store:   st,
		addr:    addr,
		cfg:     cfg,
		engine:  engine,
		updater: updater,
		log:     log,
		csrf:    randomToken(),
		tmpl:    tmpl,
		static:  http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", s.static)
	mux.HandleFunc("GET /tool/{id}", s.toolDetail)
	mux.HandleFunc("POST /tool/{id}/update", s.toolUpdate)
	mux.HandleFunc("GET /activity", s.activity)
	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /check", s.checkNow)
	mux.HandleFunc("/", s.dashboard)
	return hostGuard(mux)
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	server := &http.Server{Addr: s.addr, Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tools, err := s.toolViews(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := dashboardData{
		Tools:   tools,
		Total:   len(tools),
		Addr:    s.addr,
		CSRF:    s.csrf,
		Message: messageFor(r.URL.Query()),
		Error:   r.URL.Query().Get("error"),
	}
	for _, tool := range tools {
		switch tool.State {
		case present.StateUpdate:
			data.UpdateCount++
			if !tool.Updateable {
				data.AlertCount++
			}
		case present.StateSystem, present.StateManual:
			data.ManagedCount++
		}
	}

	if checks, err := s.store.ListChecks(r.Context(), 1); err == nil && len(checks) > 0 {
		data.LastCheck = formatTime(checks[0].FinishedAt)
		if checks[0].Status != "ok" && checks[0].Error != "" {
			data.Error = checks[0].Error
		}
	}

	s.render(w, "dashboard.html", data)
}

func (s *Server) toolDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tool, err := s.store.GetTool(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	updates, err := s.store.ListUpdates(r.Context(), id, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := toolData{
		Tool:    s.viewOf(tool),
		CSRF:    s.csrf,
		Message: messageFor(r.URL.Query()),
		Error:   r.URL.Query().Get("error"),
	}
	for _, record := range updates {
		data.Updates = append(data.Updates, updateViewOf(record))
	}
	s.render(w, "tool.html", data)
}

func (s *Server) toolUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.csrf)) != 1 {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tool, err := s.store.GetTool(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !s.cfg.Update.Enabled {
		redirectError(w, r, id, "updates are disabled in config")
		return
	}

	plan, err := s.updater.Plan(tool, "", s.cfg.Update.MinReleaseAge.Duration(), false)
	if err != nil {
		redirectError(w, r, id, err.Error())
		return
	}

	applyCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	result, err := s.updater.Apply(applyCtx, plan)
	if err != nil {
		if s.log != nil {
			s.log.Warn("update failed", "tool", tool.Name, "err", err)
		}
		redirectError(w, r, id, err.Error())
		return
	}
	if err := s.store.SetToolVersion(r.Context(), id, result.Version); err != nil && s.log != nil {
		s.log.Warn("could not store updated version", "tool", tool.Name, "err", err)
	}
	target := fmt.Sprintf("/tool/%d?updated=%s", id, url.QueryEscape(result.Version))
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) checkNow(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.csrf)) != 1 {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	if s.engine == nil {
		http.Redirect(w, r, "/?error=check+is+not+configured", http.StatusSeeOther)
		return
	}

	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Minute)
	defer cancel()
	result, err := s.engine.Run(checkCtx, true)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/?checked=%d&updates=%d", result.Tools, result.Updates), http.StatusSeeOther)
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	checks, err := s.store.ListChecks(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updates, err := s.store.ListUpdates(r.Context(), 0, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := activityData{}
	for _, record := range checks {
		data.Checks = append(data.Checks, checkView{
			Started:  formatTime(record.StartedAt),
			Finished: formatTime(record.FinishedAt),
			Status:   record.Status,
			Tools:    record.Tools,
			Updates:  record.Updates,
			Error:    record.Error,
		})
	}
	for _, record := range updates {
		data.Updates = append(data.Updates, updateViewOf(record))
	}
	s.render(w, "activity.html", data)
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	dataDir, _ := config.DataDir()
	configPath, _ := config.DefaultPath()
	data := settingsData{
		ConfigPath:    configPath,
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "toolcheck.db"),
		Addr:          s.addr,
		Interval:      s.cfg.Check.Interval.String(),
		CacheTTL:      s.cfg.Check.CacheTTL.String(),
		NodeChannel:   s.cfg.NodeChannel(),
		MinReleaseAge: s.cfg.Update.MinReleaseAge.String(),
		UpdatesOn:     s.cfg.Update.Enabled,
		CSRF:          s.csrf,
	}
	if checks, err := s.store.ListChecks(r.Context(), 1); err == nil && len(checks) > 0 {
		data.LastCheck = formatTime(checks[0].FinishedAt)
	}
	s.render(w, "settings.html", data)
}

func (s *Server) toolViews(ctx context.Context) ([]toolView, error) {
	tools, err := s.store.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]toolView, 0, len(tools))
	for _, tool := range tools {
		if !tool.Active && !s.cfg.Tools.ShowInactive {
			continue
		}
		if s.cfg.Tools.HideOSManaged && present.SystemManaged(tool) {
			continue
		}
		views = append(views, s.viewOf(tool))
	}
	sort.SliceStable(views, func(i, j int) bool {
		left, right := stateRank(views[i].State), stateRank(views[j].State)
		if left != right {
			return left < right
		}
		return views[i].Name < views[j].Name
	})
	return views, nil
}

func (s *Server) viewOf(tool store.Tool) toolView {
	latest := tool.LatestVersion
	if latest == "" {
		latest = "—"
	}
	hint := ""
	if entry, ok := registry.ByName(tool.Name); ok {
		hint = entry.Hint
	}
	view := toolView{
		ID:              tool.ID,
		Name:            tool.Name,
		Family:          tool.Family,
		Version:         tool.Version,
		Latest:          latest,
		Origin:          tool.Origin,
		ManagedBy:       tool.ManagedBy,
		Risk:            tool.Risk,
		Path:            tool.Path,
		RealPath:        tool.RealPath,
		Hint:            hint,
		State:           present.State(tool),
		UpdateAvailable: present.UpdateAvailable(tool),
		Updateable:      tool.Updateable && present.UpdateAvailable(tool) && !present.SystemManaged(tool),
		System:          present.SystemManaged(tool),
		Active:          tool.Active,
		Security:        tool.LatestSecurity,
	}
	if tool.LatestPublishedAt != nil {
		view.Published = formatTime(*tool.LatestPublishedAt)
		view.Age = humanAge(*tool.LatestPublishedAt)
	}
	return view
}

func stateRank(state string) int {
	switch state {
	case present.StateUpdate:
		return 0
	case present.StateManual:
		return 1
	case present.StateSystem:
		return 2
	default:
		return 3
	}
}

func updateViewOf(record store.UpdateRecord) updateView {
	return updateView{
		Tool:     record.ToolName,
		From:     record.FromVersion,
		To:       record.ToVersion,
		Status:   record.Status,
		Command:  record.Command,
		Output:   record.Output,
		Started:  formatTime(record.StartedAt),
		Finished: formatTime(record.FinishedAt),
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.UTC().Format("2006-01-02 15:04")
}

func humanAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	duration := time.Since(t)
	switch {
	case duration < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	case duration < 48*time.Hour:
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
}

func messageFor(query url.Values) string {
	if value := query.Get("checked"); value != "" {
		return fmt.Sprintf("checked %s tools; %s updates available", value, query.Get("updates"))
	}
	if value := query.Get("updated"); value != "" {
		return fmt.Sprintf("updated to %s", value)
	}
	return ""
}

func redirectError(w http.ResponseWriter, r *http.Request, id int64, message string) {
	if len(message) > 500 {
		message = message[:500]
	}
	http.Redirect(w, r, fmt.Sprintf("/tool/%d?error=%s", id, url.QueryEscape(message)), http.StatusSeeOther)
}

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || !loopbackHost(parsed.Host) {
					http.Error(w, "forbidden origin", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(hostport string) bool {
	host := hostport
	if parsed, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}
