package byse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/imroc/req/v3"
	"go.uber.org/zap"
)

type Options struct {
	BaseURL       string
	APIKey        string
	Timeout       time.Duration
	UploadTimeout time.Duration
}

type MediaRepository struct {
	apiKey     string
	client     *req.Client
	uploadCl   *req.Client
	uploadWait time.Duration
	ll         *zap.Logger
}

var _ domain.IByseMediaRepository = (*MediaRepository)(nil)

func (r *MediaRepository) Create(ctx context.Context, name string, body io.Reader, size int64, contentType string) (*domain.ByseFile, error) {
	ll := r.ll.Named("Create").With(zap.String("name", name))
	serverURL, err := r.uploadServer(ctx)
	if err != nil {
		return nil, err
	}
	ll.Debug("got upload endpoint", zap.String("server", serverURL))
	ll = ll.With(zap.String("endpoint", serverURL))
	if r.uploadWait > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.uploadWait)
		defer cancel()
	}
	filename := name
	if filename == "" {
		filename = "file"
	}
	resp, err := r.uploadCl.R().
		SetContext(ctx).
		SetFormData(map[string]string{"key": r.apiKey}).
		SetFileUpload(req.FileUpload{
			ParamName:   "file",
			FileName:    filename,
			ContentType: contentType,
			FileSize:    size,
			GetFileContent: func() (io.ReadCloser, error) {
				return io.NopCloser(body), nil
			},
		}).
		EnableForceChunkedEncoding().
		Post(serverURL)
	if err != nil {
		return nil, fmt.Errorf("can not upload byse file: %w", err)
	}
	raw := resp.Bytes()
	if resp.GetStatusCode() != http.StatusOK {
		return nil, fmt.Errorf("can not upload byse file: unexpected http status %d (%s)", resp.GetStatusCode(), string(raw))
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
	filename = f.Filename
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
		apiKey: opts.APIKey,
		client: req.C().
			SetBaseURL(baseURL).
			SetTimeout(timeout).
			SetCommonQueryParam("key", opts.APIKey),
		uploadCl:   req.C().SetTimeout(uploadTimeout),
		uploadWait: uploadTimeout,
		ll:         log.Named(log.REPOSITORY, "byse"),
	}, nil
}
