package pdf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/Epistemic-Technology/zotero/zotero"
)

func SplitPdf(pdf models.PdfData) (models.PdfPages, error) {
	var pages models.PdfPages
	reader := bytes.NewReader(pdf)
	conf := model.NewDefaultConfiguration()
	pdfContext, err := api.ReadValidateAndOptimize(reader, conf)
	if err != nil {
		return pages, err
	}
	pageCount := pdfContext.PageCount
	if pageCount == 0 {
		return pages, nil
	}
	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageReader, err := api.ExtractPage(pdfContext, pageNum)
		if err != nil {
			return pages, err
		}
		pageData, err := io.ReadAll(pageReader)
		if err != nil {
			return pages, err
		}
		pages = append(pages, models.PdfPageData(pageData))
	}
	return pages, nil
}

func GetData(ctx context.Context, sourceInfo models.SourceInfo) (models.PdfData, error) {
	var data models.PdfData
	var err error
	if sourceInfo.ZoteroID != "" {
		zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
		libraryID := os.Getenv("ZOTERO_LIBRARY_ID")
		data, err = GetFromZotero(ctx, sourceInfo.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			return nil, err
		}
	} else if sourceInfo.URL != "" {
		data, err = GetFromURL(ctx, sourceInfo.URL)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("no data provided")
	}

	if data == nil {
		return nil, errors.New("no data retrieved")
	}

	return data, nil
}

func GetFromURL(ctx context.Context, url string) (models.PdfData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func GetFromZotero(ctx context.Context, zoteroID string, apiKey string, libraryID string) (models.PdfData, error) {
	client := zotero.NewClient(libraryID, zotero.LibraryTypeUser, zotero.WithAPIKey(apiKey))
	data, err := client.File(ctx, zoteroID)
	if err != nil {
		return nil, err
	}
	return data, nil
}
