package gometa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

type Response struct {
	Cedula      string      `json:"cedula"`
	Nombre      string      `json:"nombre"`
	Resultcount int         `json:"resultcount"`
	Results     []Result    `json:"results"`
	Situacion   *Situacion  `json:"situacion,omitempty"`
	Regimen     *Regimen    `json:"regimen,omitempty"`
	Actividades []Actividad `json:"actividades,omitempty"`
}

type Result struct {
	Cedula     string  `json:"cedula"`
	Type       string  `json:"type"`
	Class      string  `json:"class"`
	GuessType  string  `json:"guess_type"`
	Fullname   string  `json:"fullname"`
	Firstname  *string `json:"firstname"`
	Firstname1 *string `json:"firstname1"`
	Firstname2 *string `json:"firstname2"`
	Lastname   *string `json:"lastname"`
	Lastname1  *string `json:"lastname1"`
	Lastname2  *string `json:"lastname2"`
}

type Situacion struct {
	Estado                   string `json:"estado"`
	Omiso                    string `json:"omiso"`
	Moroso                   string `json:"moroso"`
	AdministracionTributaria string `json:"administracionTributaria"`
}

type Regimen struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

type Actividad struct {
	Codigo      string `json:"codigo"`
	CIIU4       string `json:"CIIU4"`
	CIIU4Desc   string `json:"CIIU4desc"`
	Descripcion string `json:"descripcion"`
	Estado      string `json:"estado"`
	Tipo        string `json:"tipo"`
}

type Outcome int

const (
	Found Outcome = iota
	NotFound
	Offline
)

type Lookup struct {
	Outcome  Outcome
	Response *Response
	RawJSON  []byte
	Err      error
}

// Get queries GoMeta for a cedula. Distinguishes Found, NotFound, Offline.
// GoMeta returns HTTP 200 with empty results when cedula is not in the registry,
// so we use len(results) == 0 to detect NotFound (not status code).
func (c *Client) Get(ctx context.Context, cedula string) Lookup {
	url := fmt.Sprintf("%s/cedulas/%s", c.baseURL, cedula)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Lookup{Outcome: Offline, Err: err}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Lookup{Outcome: Offline, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return Lookup{Outcome: Offline, Err: fmt.Errorf("gometa http %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Lookup{Outcome: Offline, Err: err}
	}

	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		return Lookup{Outcome: Offline, Err: fmt.Errorf("gometa parse: %w", err)}
	}

	if len(r.Results) == 0 {
		return Lookup{Outcome: NotFound, RawJSON: body, Response: &r}
	}
	return Lookup{Outcome: Found, RawJSON: body, Response: &r}
}
