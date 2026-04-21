package router

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestLookupGeoIPRecordRejectsInvalidIP(t *testing.T) {
	t.Helper()

	_, err := lookupGeoIPRecord("not-an-ip")
	if err == nil {
		t.Fatal("expected invalid IP error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, httpErr.Code)
	}
}
