package main

import (
	"fmt"
	"os"

	"aladin/backend_v2/internal/ingestion"
)

func main() {
	for _, path := range os.Args[1:] {
		doc := ingestion.ExtractPDF(path)
		fmt.Printf("\n%s\n  status=%s pages=%d chars=%d sections=%d\n",
			path, doc.Status, doc.PageCount, doc.TextLen(), len(doc.Sections))
		if doc.Error != "" {
			fmt.Printf("  error: %s\n", doc.Error)
		}
		for _, s := range doc.Sections {
			fmt.Printf("    %*s%s  (p%d)\n", s.Level*4, "", s.Title, s.Page)
		}
		if len(doc.Pages) > 4 {
			fmt.Printf("  page 5 text: %.90q\n", doc.Pages[4].Text)
		}
	}
}
