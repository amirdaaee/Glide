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

	"github.com/amirdaaee/Glide/internals/domain"
)

type apiEnvelope struct {
	Msg        string          `json:"msg"`
	ServerTime string          `json:"server_time"`
	Status     int             `json:"status"`
	Result     json.RawMessage `json:"result"`
}

type fileDTO struct {
	Status    int     `json:"status"`
	FileCode  string  `json:"file_code"`
	Filecode  string  `json:"filecode"`
	Name      string  `json:"name"`
	Title     string  `json:"title"`
	CanPlay   flexInt `json:"canplay"`
	Views     flexInt `json:"views"`
	Length    flexInt `json:"length"`
	Uploaded  string  `json:"uploaded"`
	FolderID  flexInt `json:"fld_id"`
	Public    flexInt `json:"public"`
	Thumbnail string  `json:"thumbnail"`
	Link      string  `json:"link"`
	URL       string  `json:"url"`
}

func (d fileDTO) code() string {
	if d.FileCode != "" {
		return d.FileCode
	}
	return d.Filecode
}

func (d fileDTO) displayName() string {
	if d.Name != "" {
		return d.Name
	}
	return d.Title
}

func (d fileDTO) toDomain() *domain.ByseFile {
	link := d.Link
	if link == "" {
		link = d.URL
	}
	return &domain.ByseFile{
		FileCode:  d.code(),
		Name:      d.displayName(),
		CanPlay:   int(d.CanPlay) != 0,
		Duration:  int(d.Length),
		Views:     int(d.Views),
		Uploaded:  d.Uploaded,
		FolderID:  int(d.FolderID),
		Public:    int(d.Public) != 0,
		Thumbnail: d.Thumbnail,
		Link:      link,
	}
}

type flexInt int

func (v *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		str = strings.TrimSpace(str)
		if str == "" {
			*v = 0
			return nil
		}
		n, err := strconv.Atoi(str)
		if err != nil {
			return err
		}
		*v = flexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*v = flexInt(n)
	return nil
}

func (r *MediaRepository) get(ctx context.Context, path string, params url.Values, dest any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("key", r.apiKey)
	u := r.baseURL + path
	if encoded := params.Encode(); encoded != "" {
		u += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("can not build request: %w", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("can not call %s: %w", path, err)
	}
	defer resp.Body.Close()
	return decodeEnvelope(path, resp, dest)
}

func decodeEnvelope(path string, resp *http.Response, dest any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can not read %s: %w", path, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return domain.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected http status %d", path, resp.StatusCode)
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("can not decode %s: %w", path, err)
	}
	if env.Status != 0 && env.Status != http.StatusOK {
		if env.Status == http.StatusNotFound {
			return domain.ErrNotFound
		}
		msg := env.Msg
		if msg == "" {
			msg = "request failed"
		}
		return fmt.Errorf("%s: %s (status %d)", path, msg, env.Status)
	}
	if dest == nil || len(env.Result) == 0 || string(env.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Result, dest); err != nil {
		return fmt.Errorf("can not decode %s result: %w", path, err)
	}
	return nil
}
