package pdf

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/Epistemic-Technology/zotero/zotero"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
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
