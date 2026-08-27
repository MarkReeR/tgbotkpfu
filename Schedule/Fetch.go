package Schedule

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SheetSource identifies one course tab of the schedule spreadsheet.
type SheetSource struct {
	Name string // e.g. "1 курс", used only for logging/course parsing
	Gid  string // the tab's gid, visible in the browser URL as #gid=...
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// fetchSheetCSV downloads one tab of a public Google Sheet as CSV rows.
func fetchSheetCSV(spreadsheetID string, gid string) ([][]string, error) {
	url := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%s", spreadsheetID, gid)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching sheet gid=%s", resp.StatusCode, gid)
	}

	// A sheet that has been made private, moved or deleted still answers 200 -
	// with an HTML sign-in or error page. Parsed as CSV that turns into garbage
	// rows, so reject anything that is not actually CSV.
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "csv") {
		return nil, fmt.Errorf("sheet gid=%s returned %q, not CSV - is the document still public?", gid, ct)
	}

	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	rows, err := reader.ReadAll()
	if err != nil && err != io.EOF {
		return nil, err
	}
	return rows, nil
}
