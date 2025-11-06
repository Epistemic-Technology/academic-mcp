package documents

import (
	"bytes"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// SplitPdf splits a PDF document into individual pages
func SplitPdf(pdf models.DocumentData) (models.DocumentPages, error) {
	var pages models.DocumentPages
	reader := bytes.NewReader(pdf.Data)
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
		pages = append(pages, models.DocumentPageData(pageData))
	}
	return pages, nil
}
