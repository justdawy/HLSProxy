package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ProxyBase = "http://localhost:3000"

func rewriteManifest(content, proxyBase, playlistURL string) string {
	base, err := url.Parse(playlistURL)
	if err != nil {
		return content
	}

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			lines[i] = line
			continue
		}

		ref, err := url.Parse(line)
		if err != nil {
			continue
		}

		resolved := base.ResolveReference(ref).String()
		encoded := url.QueryEscape(resolved)

		lines[i] = proxyBase + "/proxy?url=" + encoded
	}

	return strings.Join(lines, "\n")
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "Missing url", http.StatusBadRequest)
		return
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Proxy error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", contentType)

	if strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
		strings.HasSuffix(target, ".m3u8") {

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}

		rewritten := rewriteManifest(
			string(bodyBytes),
			ProxyBase,
			target,
		)

		w.Write([]byte(rewritten))
		return
	}

	io.Copy(w, resp.Body)
}

func main() {
	http.HandleFunc("/proxy", proxyHandler)

	fmt.Println("HLS proxy running on port 3000")
	http.ListenAndServe(":3000", nil)
}
