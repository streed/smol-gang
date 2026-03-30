package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/models"
)

// AppProxy reverse-proxies HTTP and WebSocket traffic to a running app inside an agent pod.
// Route: /api/v1/workstreams/{id}/app/{port}/*
func (h *WorkstreamHandler) AppProxy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	portStr := chi.URLParam(r, "port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid port"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.ServiceName == "" {
		writeJSON(w, http.StatusServiceUnavailable, models.ErrorResponse{Error: "workstream has no service"})
		return
	}

	// Validate the requested port is in the workstream's port mappings
	if !isPortAllowed(workstream.PortMappings, port) {
		writeJSON(w, http.StatusForbidden, models.ErrorResponse{Error: "port not configured for this workstream"})
		return
	}

	targetHost := h.K8s.GetServiceHost(workstream.ServiceName, port)

	// WebSocket upgrade — hijack and bidirectionally pipe
	if isWebSocketUpgrade(r) {
		h.proxyWebSocket(w, r, targetHost, id.String(), port)
		return
	}

	// Standard HTTP reverse proxy
	target, _ := url.Parse(fmt.Sprintf("http://%s", targetHost))
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1 // stream SSE immediately

	// Rewrite request path: strip the /api/v1/workstreams/{id}/app/{port} prefix
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// chi stores the wildcard path in the URL after the route pattern
		remaining := chi.URLParam(r, "*")
		if remaining == "" {
			remaining = "/"
		} else if !strings.HasPrefix(remaining, "/") {
			remaining = "/" + remaining
		}
		req.URL.Path = remaining
		req.URL.RawPath = ""
		req.Host = target.Host
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Host", r.Host)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: fmt.Sprintf("app unreachable: %v", err)})
	}

	proxy.ServeHTTP(w, r)
}

func (h *WorkstreamHandler) proxyWebSocket(w http.ResponseWriter, r *http.Request, targetHost, wsID string, port int) {
	// Dial the backend
	backendConn, err := net.Dial("tcp", targetHost)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "failed to connect to app"})
		return
	}
	defer backendConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "hijack not supported"})
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "hijack failed"})
		return
	}
	defer clientConn.Close()

	// Forward the original upgrade request to the backend
	remaining := chi.URLParam(r, "*")
	if remaining == "" {
		remaining = "/"
	} else if !strings.HasPrefix(remaining, "/") {
		remaining = "/" + remaining
	}
	r.URL.Path = remaining
	r.URL.RawPath = ""
	r.RequestURI = remaining
	if r.URL.RawQuery != "" {
		r.RequestURI = remaining + "?" + r.URL.RawQuery
	}
	r.Write(backendConn)

	// Bidirectional copy
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(backendConn, clientBuf)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()
	<-done
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func isPortAllowed(portMappings []models.PortMapping, port int) bool {
	if len(portMappings) == 0 {
		// No port mappings configured — allow any port (permissive for legacy mode)
		return true
	}
	for _, pm := range portMappings {
		if pm.ContainerPort == port {
			return true
		}
	}
	return false
}

// inCluster returns true if the gateway is running inside the K8s cluster.
func inCluster() bool {
	return os.Getenv("K8S_IN_CLUSTER") == "true"
}
