package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"
	"github.com/yourusername/restaurant-finance/internal/core"
)

const maxPOSResponseBytes = 25 << 20

type HTTPPOSAdapter struct {
	provider string
	client   *http.Client
}

func NewHTTPPOSAdapter(provider string) *HTTPPOSAdapter {
	return &HTTPPOSAdapter{
		provider: provider,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (a *HTTPPOSAdapter) Provider() string { return a.provider }

func (a *HTTPPOSAdapter) Test(ctx context.Context, connection core.POSConnection) error {
	resp, err := a.request(ctx, connection, connection.BaseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return statusError(a.provider, resp.StatusCode)
}

func (a *HTTPPOSAdapter) Fetch(ctx context.Context, connection core.POSConnection, from, to time.Time) ([]core.FinancialEntry, error) {
	endpoint, err := validatedURL(connection.BaseURL)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("from", from.Format(time.DateOnly))
	query.Set("to", to.Format(time.DateOnly))
	endpoint.RawQuery = query.Encode()

	resp, err := a.request(ctx, connection, endpoint.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := statusError(a.provider, resp.StatusCode); err != nil {
		return nil, err
	}

	var rows []posRow
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxPOSResponseBytes))
	if err := decoder.Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", a.provider, err)
	}
	if len(rows) > 100_000 {
		return nil, errors.New("POS response exceeds 100000 rows")
	}

	result := make([]core.FinancialEntry, 0, len(rows))
	for index, row := range rows {
		if err := row.validate(); err != nil {
			return nil, fmt.Errorf("POS row %d: %w", index+1, err)
		}
		result = append(result, core.FinancialEntry{
			RestaurantID:  connection.RestaurantID,
			OccurredAt:    row.Date,
			Amount:        row.Amount.Abs(),
			Direction:     row.Direction,
			PaymentMethod: row.PaymentMethod,
			Description:   row.Description,
			Source:        a.provider,
			ExternalID:    row.ID,
		})
	}
	return result, nil
}

func (a *HTTPPOSAdapter) request(ctx context.Context, connection core.POSConnection, endpoint string) (*http.Response, error) {
	if _, err := validatedURL(endpoint); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if connection.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+connection.APIKey)
	}
	return a.client.Do(req)
}

func validatedURL(value string) (*url.URL, error) {
	endpoint, err := url.ParseRequestURI(value)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, errors.New("POS base_url must be a valid HTTP(S) URL")
	}
	return endpoint, nil
}

func statusError(provider string, status int) error {
	if status < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("%s returned HTTP %d", provider, status)
}

type posRow struct {
	ID            string          `json:"id"`
	Date          time.Time       `json:"date"`
	Amount        decimal.Decimal `json:"amount"`
	Direction     string          `json:"direction"`
	PaymentMethod string          `json:"payment_method"`
	Description   string          `json:"description"`
}

func (r posRow) validate() error {
	switch {
	case r.ID == "":
		return errors.New("id is required for deduplication")
	case r.Date.IsZero():
		return errors.New("date is required")
	case r.Amount.LessThanOrEqual(decimal.Zero):
		return errors.New("amount must be positive")
	case r.Direction != "income" && r.Direction != "expense":
		return errors.New("direction must be income or expense")
	default:
		return nil
	}
}
