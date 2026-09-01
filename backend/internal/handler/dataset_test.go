package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/raids-lab/crater/dao/model"
)

func TestBindDatasetListPageQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantPage   int
		wantSize   int
		wantSearch string
		wantSort   string
		wantType   model.DataType
		wantErr    bool
	}{
		{
			name:     "defaults",
			query:    "",
			wantPage: 1,
			wantSize: 10,
			wantSort: "-createdAt",
			wantType: model.DataTypeShareFile,
		},
		{
			name:       "normalizes supported query",
			query:      "page=2&page_size=20&search=%E6%B5%8B%E8%AF%95&owner=mine&sort=-mountCount&type=sharefile",
			wantPage:   2,
			wantSize:   20,
			wantSearch: "测试",
			wantSort:   "-mountCount",
			wantType:   model.DataTypeShareFile,
		},
		{
			name:    "rejects unsupported sort",
			query:   "sort=name",
			wantErr: true,
		},
		{
			name:    "rejects oversized search",
			query:   "search=" + strings.Repeat("a", datasetMaxSearchRunes+1),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", "/api/v1/dataset/mydataset/page?"+tt.query, nil)

			got, err := bindDatasetListPageQuery(context)
			if (err != nil) != tt.wantErr {
				t.Fatalf("bindDatasetListPageQuery() error = %v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Page != tt.wantPage || got.PageSize != tt.wantSize || got.Search != tt.wantSearch ||
				got.Sort != tt.wantSort || got.Type != tt.wantType {
				t.Fatalf("unexpected query: %#v", got)
			}
		})
	}
}

func TestNormalizedOrganizationLogoKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		organization string
		repositoryID string
		want         string
	}{
		{name: "explicit organization", organization: " Qwen ", repositoryID: "ignored", want: "qwen"},
		{name: "legacy repository", repositoryID: "qwen/qwen2-0.5b", want: "qwen"},
		{name: "manual resource name", repositoryID: "qwen2-0.5b", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizedOrganizationLogoKey(tt.organization, tt.repositoryID); got != tt.want {
				t.Fatalf("normalizedOrganizationLogoKey(%q, %q) = %q, want %q", tt.organization, tt.repositoryID, got, tt.want)
			}
		})
	}
}

func TestDeduplicateDatasets(t *testing.T) {
	t.Parallel()

	newerDuplicate := &model.Dataset{Name: "SKYLENAGE/SkyJM-Gen-4B", Type: model.DataTypeModel}
	newerDuplicate.ID = 92
	original := &model.Dataset{Name: "skylenage/SkyJM-Gen-4B", Type: model.DataTypeModel}
	original.ID = 91
	datasetWithSameName := &model.Dataset{Name: "SKYLENAGE/SkyJM-Gen-4B", Type: model.DataTypeDataset}
	datasetWithSameName.ID = 93

	downloadedKeys := map[string]struct{}{
		resourceMetadataKey("skylenage/skyjm-gen-4b", string(model.DataTypeModel)): {},
	}
	got := deduplicateDownloadedDatasets(
		[]*model.Dataset{newerDuplicate, original, datasetWithSameName}, downloadedKeys,
	)
	if len(got) != 2 {
		t.Fatalf("deduplicateDatasets() returned %d rows, want 2", len(got))
	}
	if got[0].ID != original.ID {
		t.Fatalf("deduplicateDatasets() kept ID %d, want canonical ID %d", got[0].ID, original.ID)
	}
	if got[1].ID != datasetWithSameName.ID {
		t.Fatalf("deduplicateDatasets() incorrectly merged different resource types")
	}
}

func TestDeduplicateDownloadedDatasetsPreservesUserResources(t *testing.T) {
	t.Parallel()

	first := &model.Dataset{Name: "experiment", Type: model.DataTypeDataset}
	first.ID = 1
	second := &model.Dataset{Name: "experiment", Type: model.DataTypeDataset}
	second.ID = 2

	got := deduplicateDownloadedDatasets([]*model.Dataset{first, second}, nil)
	if len(got) != 2 {
		t.Fatalf("deduplicateDownloadedDatasets() merged user resources, got %d rows", len(got))
	}
}
