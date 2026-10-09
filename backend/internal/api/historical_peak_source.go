package api

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxHistoricalPeakSourceBytes = 10 << 20

type uploadHistoricalPeakSourceInput struct {
	Asset    string `json:"asset"`
	Filename string `json:"filename"`
	CSV      string `json:"csv"`
}

type historicalPeakSourceUpload struct {
	SourceFile string `json:"sourceFile"`
}

func (s *Server) handleUploadHistoricalPeakSource(w http.ResponseWriter, r *http.Request) {
	var input uploadHistoricalPeakSourceInput
	if err := decodeJSONLimit(r, &input, maxHistoricalPeakSourceBytes); err != nil {
		writeDecodeError(w, err)
		return
	}
	asset, ok := validAsset(input.Asset)
	if !ok {
		writeError(w, 400, "asset must be BTC or ETH")
		return
	}
	source, err := saveHistoricalPeakSource(s.cfg.DatabasePath, asset, input.Filename, []byte(input.CSV))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, historicalPeakSourceUpload{SourceFile: source})
}

// saveHistoricalPeakSource stores an uploaded CSV under a content-addressed
// filename. Selecting the same file again cannot overwrite a different file.
func saveHistoricalPeakSource(databasePath, asset, filename string, contents []byte) (string, error) {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return "", fmt.Errorf("CSV filename is required")
	}
	if len(contents) == 0 {
		return "", fmt.Errorf("CSV file is required")
	}
	rows, err := csv.NewReader(strings.NewReader(string(contents))).ReadAll()
	if err != nil || len(rows) < 2 || len(rows[0]) < 6 || strings.Join(rows[0][:6], ",") != "timestamp,open,high,low,close,volume" {
		return "", fmt.Errorf("historical peak source must be a daily OHLCV CSV with timestamp,open,high,low,close,volume columns")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(contents))
	directory := filepath.Join(filepath.Dir(databasePath), "historical-peak-sources")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create historical peak source directory: %w", err)
	}
	path := filepath.Join(directory, strings.ToLower(asset)+"-"+hash[:12]+"-"+filename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("save historical peak source: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return "", fmt.Errorf("write historical peak source: %w", err)
	}
	return path, nil
}
