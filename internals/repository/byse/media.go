package byse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
)

type Options struct {
	BaseURL       string
	APIKey        string
	Timeout       time.Duration
	UploadTimeout time.Duration
}

type MediaRepository struct {
	baseURL    string
	apiKey     string
	client     *http.Client
	uploadCl   *http.Client
	uploadWait time.Duration
}

var _ domain.IByseMediaRepository = (*MediaRepository)(nil)

func (r *MediaRepository) Create(ctx context.Context, name string, body io.Reader, size int64, contentType string) (*domain.ByseFile, error) {
	serverURL, err := r.uploadServer(ctx)
	if err != nil {
		return nil, err
	}
	if r.uploadWait > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.uploadWait)
		defer cancel()
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeUploadForm(mw, r.apiKey, name, contentType, body))
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, pr)
	if err != nil {
		_ = pw.CloseWithError(err)
		return nil, fmt.Errorf("can not build byse upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := r.uploadCl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can not upload byse file: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can not read byse upload response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("can not upload byse file: unexpected http status %d", resp.StatusCode)
	}
	var out struct {
		Msg    string `json:"msg"`
		Status int    `json:"status"`
		Files  []struct {
			Filecode string `json:"filecode"`
			Filename string `json:"filename"`
			Status   string `json:"status"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("can not decode byse upload response: %w", err)
	}
	if out.Status != 0 && out.Status != http.StatusOK {
		msg := out.Msg
		if msg == "" {
			msg = "upload failed"
		}
		return nil, fmt.Errorf("can not upload byse file: %s (status %d)", msg, out.Status)
	}
	if len(out.Files) == 0 {
		return nil, fmt.Errorf("can not upload byse file: empty files in response")
	}
	f := out.Files[0]
	if f.Status != "" && !strings.EqualFold(f.Status, "OK") {
		return nil, fmt.Errorf("can not upload byse file: %s", f.Status)
	}
	filename := f.Filename
	if filename == "" {
		filename = name
	}
	return &domain.ByseFile{FileCode: f.Filecode, Name: filename}, nil
}

func (r *MediaRepository) Get(ctx context.Context, fileCode string) (*domain.ByseFile, error) {
	if fileCode == "" {
		return nil, fmt.Errorf("file code is empty")
	}
	params := url.Values{}
	params.Set("file_code", fileCode)
	var files []fileDTO
	if err := r.get(ctx, "/file/info", params, &files); err != nil {
		return nil, fmt.Errorf("can not get byse file: %w", err)
	}
	if len(files) == 0 {
		return nil, domain.ErrNotFound
	}
	f := files[0]
	if f.Status == http.StatusNotFound {
		return nil, domain.ErrNotFound
	}
	if f.Status != 0 && f.Status != http.StatusOK {
		return nil, fmt.Errorf("can not get byse file: status %d", f.Status)
	}
	return f.toDomain(), nil
}

func (r *MediaRepository) List(ctx context.Context, filter domain.ByseListFilter) ([]*domain.ByseFile, error) {
	params := url.Values{}
	if filter.FolderID != nil {
		params.Set("fld_id", strconv.Itoa(*filter.FolderID))
	}
	if filter.Title != "" {
		params.Set("title", filter.Title)
	}
	if filter.Created != "" {
		params.Set("created", filter.Created)
	}
	if filter.Public != nil {
		if *filter.Public {
			params.Set("public", "1")
		} else {
			params.Set("public", "0")
		}
	}
	if filter.PerPage > 0 {
		params.Set("per_page", strconv.Itoa(filter.PerPage))
	}
	if filter.Page > 0 {
		params.Set("page", strconv.Itoa(filter.Page))
	}
	var files []fileDTO
	if err := r.get(ctx, "/file/list", params, &files); err != nil {
		return nil, fmt.Errorf("can not list byse files: %w", err)
	}
	out := make([]*domain.ByseFile, 0, len(files))
	for _, f := range files {
		if f.Status == http.StatusNotFound {
			continue
		}
		out = append(out, f.toDomain())
	}
	return out, nil
}

func (r *MediaRepository) SetFolder(ctx context.Context, fileCode string, folderID int) error {
	if fileCode == "" {
		return fmt.Errorf("file code is empty")
	}
	params := url.Values{}
	params.Set("file_code", fileCode)
	params.Set("fld_id", strconv.Itoa(folderID))
	if err := r.get(ctx, "/file/setfld", params, nil); err != nil {
		return fmt.Errorf("can not set byse file folder: %w", err)
	}
	return nil
}

func (r *MediaRepository) Clone(ctx context.Context, fileCode string) (*domain.ByseFile, error) {
	if fileCode == "" {
		return nil, fmt.Errorf("file code is empty")
	}
	params := url.Values{}
	params.Set("file_code", fileCode)
	var out struct {
		FileCode string `json:"file_code"`
		URL      string `json:"url"`
	}
	if err := r.get(ctx, "/file/clone", params, &out); err != nil {
		return nil, fmt.Errorf("can not clone byse file: %w", err)
	}
	if out.FileCode == "" {
		return nil, fmt.Errorf("can not clone byse file: empty file_code")
	}
	return &domain.ByseFile{FileCode: out.FileCode, Link: out.URL}, nil
}

func (r *MediaRepository) uploadServer(ctx context.Context) (string, error) {
	var server string
	if err := r.get(ctx, "/upload/server", nil, &server); err != nil {
		return "", fmt.Errorf("can not get byse upload server: %w", err)
	}
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("can not get byse upload server: empty result")
	}
	return server, nil
}

func writeUploadForm(mw *multipart.Writer, apiKey, name, contentType string, body io.Reader) error {
	if err := mw.WriteField("key", apiKey); err != nil {
		return err
	}
	filename := name
	if filename == "" {
		filename = "file"
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(filename)))
	if contentType != "" {
		hdr.Set("Content-Type", contentType)
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, body); err != nil {
		return err
	}
	return mw.Close()
}

func escapeQuotes(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

func NewMediaRepository(opts Options) (domain.IByseMediaRepository, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("byse api key is empty")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.byse.sx"
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	uploadTimeout := opts.UploadTimeout
	if uploadTimeout == 0 {
		uploadTimeout = 30 * time.Minute
	}
	return &MediaRepository{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		client:     &http.Client{Timeout: timeout},
		uploadCl:   &http.Client{Timeout: uploadTimeout},
		uploadWait: uploadTimeout,
	}, nil
}
