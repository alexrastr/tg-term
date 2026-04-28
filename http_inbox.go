package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPInboxMessage struct {
	From  string
	Text  string
	Alarm bool
}

type httpInboxPayload struct {
	From    string `json:"from"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Message string `json:"message"`
	Alarm   bool   `json:"alarm"`
}

func startHTTPInbox(ctx context.Context, config *Config, incoming chan<- HTTPInboxMessage, errCh chan<- error) error {
	listenAddr := strings.TrimSpace(config.HTTPListenAddr)
	if listenAddr == "" {
		return nil
	}

	path := strings.TrimSpace(config.HTTPInboxPath)
	if path == "" {
		path = "/message"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorizedHTTPInboxRequest(r, config.HTTPInboxToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		msg, err := decodeHTTPInboxMessage(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		select {
		case incoming <- msg:
		case <-ctx.Done():
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	})

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP inbox stopped: %w", err)
		}
	}()

	return nil
}

func authorizedHTTPInboxRequest(r *http.Request, expectedToken string) bool {
	expectedToken = strings.TrimSpace(expectedToken)
	if expectedToken == "" {
		return true
	}

	candidates := []string{
		strings.TrimSpace(r.Header.Get("X-Auth-Token")),
		strings.TrimSpace(r.URL.Query().Get("token")),
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if bearerToken, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		candidates = append(candidates, strings.TrimSpace(bearerToken))
	}

	for _, candidate := range candidates {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expectedToken)) == 1 {
			return true
		}
	}

	return false
}

func decodeHTTPInboxMessage(r *http.Request) (HTTPInboxMessage, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		return HTTPInboxMessage{}, fmt.Errorf("failed to read request body: %w", err)
	}

	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		return HTTPInboxMessage{}, fmt.Errorf("message body is empty")
	}

	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var payload httpInboxPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return HTTPInboxMessage{}, fmt.Errorf("invalid JSON: %w", err)
		}

		text := strings.TrimSpace(payload.Text)
		if text == "" {
			text = strings.TrimSpace(payload.Message)
		}
		if text == "" {
			return HTTPInboxMessage{}, fmt.Errorf("JSON payload must contain text or message")
		}

		from := strings.TrimSpace(payload.From)
		if from == "" {
			from = strings.TrimSpace(payload.Title)
		}

		return HTTPInboxMessage{
			From:  from,
			Text:  text,
			Alarm: payload.Alarm,
		}, nil
	}

	return HTTPInboxMessage{
		From: "HTTP",
		Text: bodyText,
	}, nil
}
