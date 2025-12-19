package share

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bosbaber/hackweek/selfstack/internal/ledger"
	"github.com/bosbaber/hackweek/selfstack/internal/policy"
	"github.com/bosbaber/hackweek/selfstack/internal/share/store"
	"github.com/bosbaber/hackweek/selfstack/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// Dummy S3 client
type dummyS3 struct{ exists bool }

func (d *dummyS3) ObjectExists(ctx context.Context, userHash, uploadID string) (bool, error) {
	return d.exists, nil
}
func (d *dummyS3) GetObject(ctx context.Context, userHash, uploadID string) (io.ReadCloser, storage.ObjectMetadata, error) {
	r := &dummyReadCloser{}
	m := storage.ObjectMetadata{ContentType: "application/octet-stream", ContentLength: "10", Filename: "file.txt"}
	return r, m, nil
}

type dummyReadCloser struct{}

func (d *dummyReadCloser) Close() error { return nil }
func (d *dummyReadCloser) Read(p []byte) (int, error) {
	n := copy(p, []byte("hello"))
	return n, io.EOF
}

// Dummy ledger/policy
type dummyLedger struct{}

func (d *dummyLedger) Balance(userID string) int64           { return 100000 }
func (d *dummyLedger) Debit(userID string, amt int64) error  { return nil }
func (d *dummyLedger) Credit(userID string, amt int64) error { return nil }
func (d *dummyLedger) ListAll() (map[string]int64, error) {
	return map[string]int64{"user1": 100000}, nil
}
func (d *dummyLedger) RecordTransaction(userID string, txn ledger.Transaction) error {
	return nil
}
func (d *dummyLedger) GetTransactionHistory(userID string, limit int) ([]ledger.Transaction, error) {
	return nil, nil
}

type dummyPolicy struct{}

func (d *dummyPolicy) CanDownload(balance int64) bool { return true }

func TestHandleCreateShare_and_Redeem(t *testing.T) {
	e := echo.New()
	mem := store.NewMemory()
	s3 := &dummyS3{}
	l := &dummyLedger{}
	p := policy.New(0)
	// Pass nil for s3 to trigger test logic in handlers
	e.POST("/files/:uploadId/share", func(c echo.Context) error {
		c.Set("userID", "user1")
		return HandleCreateShare(mem, nil)(c)
	})
	e.GET("/share/:token", HandleRedeemShare(mem, l, p, nil, 10000))

	s3.exists = true
	req := httptest.NewRequest(http.MethodPost, "/files/file1/share", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("userID", "user1")
	c.SetParamNames("uploadId")
	c.SetParamValues("file1")
	err := HandleCreateShare(mem, nil)(c)
	require.NoError(t, err)
	if rec.Code != http.StatusOK {
		t.Logf("Response body: %s", rec.Body.String())
	}
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "token")

	type resp struct {
		Token string `json:"token"`
	}
	var r resp
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	token := r.Token
	req2 := httptest.NewRequest(http.MethodGet, "/share/"+token, nil)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("token")
	c2.SetParamValues(token)
	err = HandleRedeemShare(mem, l, p, nil, 10000)(c2)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec2.Code)
}
