package handler

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindResourcePageQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/?page=2&page_size=20&search=%20gpu%20&type=gpu&type=vgpu&sort=-amount,name",
		nil,
	)

	request, resourceTypes, err := bindResourcePageQuery(ctx)
	if err != nil {
		t.Fatalf("bindResourcePageQuery() error = %v", err)
	}
	if request.Page != 2 || request.PageSize != 20 || request.Search != "gpu" {
		t.Fatalf("unexpected request: %+v", request)
	}
	if !reflect.DeepEqual(resourceTypes, []string{"gpu", "vgpu"}) {
		t.Fatalf("resourceTypes = %v", resourceTypes)
	}
}

func TestBindResourcePageQueryRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"page=0",
		"page_size=201",
		"type=unknown",
		"sort=unitPrice",
		"sort=name,name",
	}
	for _, rawQuery := range tests {
		t.Run(rawQuery, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
			if _, _, err := bindResourcePageQuery(ctx); err == nil {
				t.Fatal("bindResourcePageQuery() error = nil, want validation error")
			}
		})
	}
}

func TestResourcePageSortClausesAlwaysHaveTieBreaker(t *testing.T) {
	got := resourcePageSortClauses("-amount,name")
	want := []string{"amount DESC", "resource_name ASC", "id ASC"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resourcePageSortClauses() = %v, want %v", got, want)
	}
}

func TestBindResourcePriceIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/?resource_id=2&resource_id=5&resource_id=2",
		nil,
	)
	got, err := bindResourcePriceIDs(ctx)
	if err != nil {
		t.Fatalf("bindResourcePriceIDs() error = %v", err)
	}
	if !reflect.DeepEqual(got, []uint{2, 5}) {
		t.Fatalf("bindResourcePriceIDs() = %v", got)
	}
}
