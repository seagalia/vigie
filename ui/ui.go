// Package ui serves the page of vigie at /: the configured services and the endpoints each one gives right now.
package ui

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/seagalia/vigie/config"
)

//go:embed index.gohtml style.css page.js brand
var files embed.FS

var pageTemplate = template.Must(template.ParseFS(files, "index.gohtml"))

// Row is one service of the page, its endpoints are fetched by the browser from /services
type Row struct {
	Name        string
	Kind        string
	Source      string
	DefaultPort uint32
}

type Page struct {
	version string
	// a reload replaces the rows while the page is rendered
	rows atomic.Pointer[[]Row]
}

func NewPage(svcsConfig []*config.Service, version string) *Page {
	p := &Page{version: version}
	p.LoadServices(svcsConfig)
	return p
}

func (p *Page) LoadServices(svcsConfig []*config.Service) {
	rows := make([]Row, 0, len(svcsConfig))
	for _, svc := range svcsConfig {
		kind, source := describeSource(svc)
		rows = append(rows, Row{Name: svc.Name, Kind: kind, Source: source, DefaultPort: svc.DefaultPort})
	}
	p.rows.Store(&rows)
}

// describeSource tells where a service reads its endpoints; the url loses its user info and the kubeconfig path is
// left out, the page shows no secret
func describeSource(svc *config.Service) (string, string) {
	if svc.Kubernetes != nil {
		source := svc.Kubernetes.Namespace + "/" + svc.Kubernetes.Service
		if svc.Kubernetes.PortName != "" {
			source += ":" + svc.Kubernetes.PortName
		}
		return "kubernetes", source
	}
	if svc.DNS != nil {
		kind := "dns " + strings.ToLower(svc.DNS.Type)
		name := svc.DNS.Name
		if svc.DNS.Type == "SRV" {
			name = "_" + svc.DNS.SrvService + "._" + svc.DNS.SrvProto + "." + svc.DNS.Name
		}
		if svc.DNS.Resolver != "" {
			name += " @" + svc.DNS.Resolver
		}
		return kind, name
	}
	if svc.File != nil {
		return "file", svc.File.Path
	}
	shown := *svc.URL.UrlParsed
	shown.User = nil
	return "url", shown.String()
}

func (p *Page) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", p.serveIndex)
	fileServer := http.FileServerFS(files)
	mux.Handle("GET /style.css", fileServer)
	mux.Handle("GET /page.js", fileServer)
	mux.Handle("GET /brand/", fileServer)
}

func (p *Page) serveIndex(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Version string
		Rows    []Row
	}{Version: p.version, Rows: *p.rows.Load()}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := pageTemplate.Execute(w, data)
	if err != nil {
		http.Error(w, "render page: "+err.Error(), http.StatusInternalServerError)
	}
}
