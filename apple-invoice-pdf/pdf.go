package apple

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------------
// HTML → PDF conversion
// ---------------------------------------------------------------------------

// convertHTMLToPDF renders HTML to an A4 PDF using headless Chrome.
func convertHTMLToPDF(htmlContent string) ([]byte, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			ft, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(ft.Frame.ID, htmlContent).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithPaperWidth(8.27).
				WithPaperHeight(11.69).
				WithPrintBackground(true).
				Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("generating PDF: %w", err)
	}
	return buf, nil
}

// cleanHTML removes unwanted elements from invoice HTML and embeds external
// images as base64 data URIs so they render correctly in the PDF.
func cleanHTML(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && strings.HasPrefix(src, "http") {
			if dataURI, err := embedImage(src); err == nil {
				s.SetAttr("src", dataURI)
			}
		}
	})

	doc.Find(".action-button-cell").Remove()
	doc.Find("#footer_section > p").First().Remove()
	doc.Find("#footer_section > .custom-1sstyyn").Remove()
	doc.Find(".footer-copy p").Each(func(_ int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "UID-Nr") {
			s.SetAttr("style", "font-weight:600")
		}
	})
	doc.Find(".inline-link-group").Remove()

	html, err := doc.Html()
	if err != nil {
		return "", fmt.Errorf("rendering HTML: %w", err)
	}
	return html, nil
}

// embedImage downloads an image URL and returns it as a base64 data URI.
func embedImage(imgURL string) (string, error) {
	resp, err := http.Get(imgURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}
