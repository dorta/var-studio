package commits

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Reference struct {
	Name        string
	Revision    string
	RevisionURL string
}

type Metadata struct {
	Name      string    `json:"name"`
	Revision  string    `json:"revision"`
	Author    string    `json:"author,omitempty"`
	Email     string    `json:"email,omitempty"`
	Date      time.Time `json:"date,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Available bool      `json:"available"`
	Message   string    `json:"message,omitempty"`
}

type cacheEntry struct {
	metadata Metadata
	expires  time.Time
}

type Resolver struct {
	client *http.Client
	mu     sync.RWMutex
	cache  map[string]cacheEntry
}

func New() *Resolver {
	allowedRedirect := func(request *http.Request, _ []*http.Request) error {
		if !allowedHost(request.URL.Hostname()) {
			return fmt.Errorf(
				"redirect host is not allowed",
			)
		}
		return nil
	}
	return &Resolver{
		client: &http.Client{
			Timeout:       8 * time.Second,
			CheckRedirect: allowedRedirect,
		},
		cache: make(map[string]cacheEntry),
	}
}

func (r *Resolver) ResolveAll(
	ctx context.Context,
	references []Reference,
) []Metadata {
	result := make([]Metadata, len(references))
	firstByURL := make(map[string]int, len(references))
	unique := make([]int, 0, len(references))
	for index, reference := range references {
		key := reference.RevisionURL
		if _, exists := firstByURL[key]; exists {
			continue
		}
		firstByURL[key] = index
		unique = append(unique, index)
	}
	semaphore := make(chan struct{}, 6)
	var group sync.WaitGroup
	for _, index := range unique {
		index, reference := index, references[index]
		group.Add(1)
		go func() {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result[index] = r.resolve(ctx, reference)
		}()
	}
	group.Wait()
	for index, reference := range references {
		first := firstByURL[reference.RevisionURL]
		if first == index {
			continue
		}
		result[index] = result[first]
		result[index].Name = reference.Name
		result[index].Revision = reference.Revision
	}
	return result
}

func (r *Resolver) resolve(
	ctx context.Context,
	reference Reference,
) Metadata {
	base := Metadata{
		Name:     reference.Name,
		Revision: reference.Revision,
	}
	patch, ok := patchURL(reference.RevisionURL)
	if !ok {
		base.Message = "Commit source is not mapped"
		return base
	}
	if cached, ok := r.cached(patch); ok {
		cached.Name, cached.Revision = reference.Name, reference.Revision
		return cached
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		patch,
		nil,
	)
	if err != nil {
		base.Message = "Commit metadata is unavailable"
		return base
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", "VAR-Studio/1.0")
	response, err := r.client.Do(request)
	if err != nil {
		base.Message = "Offline or commit host unavailable"
		r.store(patch, base, 15*time.Minute)
		return base
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		base.Message = fmt.Sprintf(
			"Commit host returned HTTP %d",
			response.StatusCode,
		)
		r.store(patch, base, 15*time.Minute)
		return base
	}
	parsed, err := parsePatch(
		io.LimitReader(response.Body, 512<<10),
	)
	if err != nil {
		base.Message = "Commit metadata could not be parsed"
		r.store(patch, base, 15*time.Minute)
		return base
	}
	parsed.Name, parsed.Revision, parsed.Available = reference.Name,
		reference.Revision, true
	r.store(patch, parsed, 24*time.Hour)
	return parsed
}

func (r *Resolver) cached(key string) (Metadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.cache[key]
	return entry.metadata, ok &&
		time.Now().Before(entry.expires)
}

func (r *Resolver) store(
	key string,
	metadata Metadata,
	duration time.Duration,
) {
	r.mu.Lock()
	r.cache[key] = cacheEntry{
		metadata: metadata,
		expires:  time.Now().Add(duration),
	}
	r.mu.Unlock()
}

func patchURL(revisionURL string) (string, bool) {
	parsed, err := url.Parse(revisionURL)
	if err != nil || parsed.Scheme != "https" ||
		!allowedHost(parsed.Hostname()) {
		return "", false
	}
	switch parsed.Hostname() {
	case "github.com":
		if !strings.Contains(parsed.Path, "/commit/") {
			return "", false
		}
		parsed.RawQuery = ""
		parsed.Path += ".patch"
	case "git.yoctoproject.org":
		if !strings.Contains(parsed.Path, "/commit/") ||
			parsed.Query().Get("id") == "" {
			return "", false
		}
		parsed.Path = strings.Replace(
			parsed.Path,
			"/commit/",
			"/patch/",
			1,
		)
	default:
		return "", false
	}
	return parsed.String(), true
}

func allowedHost(host string) bool {
	switch strings.ToLower(host) {
	case "github.com",
		"patch-diff.githubusercontent.com",
		"git.yoctoproject.org":
		return true
	default:
		return false
	}
}

func parsePatch(reader io.Reader) (Metadata, error) {
	buffered := bufio.NewReader(reader)
	if _, err := buffered.ReadString('\n'); err != nil {
		return Metadata{}, err
	}
	headers, err := textproto.NewReader(buffered).
		ReadMIMEHeader()
	if err != nil {
		return Metadata{}, err
	}
	result := Metadata{}
	if address, err := mail.ParseAddress(headers.Get("From")); err == nil {
		result.Author = address.Name
		result.Email = address.Address
	}
	if parsed, err := mail.ParseDate(headers.Get("Date")); err == nil {
		result.Date = parsed.UTC()
	}
	if decoded, err := new(mime.WordDecoder).DecodeHeader(headers.Get(
		"Subject")); err == nil {
		result.Subject = strings.TrimSpace(
			strings.TrimPrefix(decoded, "[PATCH]"),
		)
	}
	if result.Author == "" || result.Date.IsZero() {
		return Metadata{}, fmt.Errorf(
			"required patch headers are missing",
		)
	}
	return result, nil
}
