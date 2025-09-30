package main

import (
	"fmt"
	"strings"
)

// CalculateStats calculates statistics from records to be deleted
func CalculateStats(records []DeletedRecord) *Stats {
	detailIDs := make(map[int]bool)
	headerIDs := make(map[int]bool)
	
	for _, record := range records {
		detailIDs[record.DetailID] = true
		headerIDs[record.HeaderID] = true
	}

	return &Stats{
		JournalRecords: len(records),
		DetailRecords:  len(detailIDs),
		HeaderRecords:  len(headerIDs),
	}
}

// ShowDryRunResults displays what would be deleted in dry run mode
func ShowDryRunResults(records []DeletedRecord) {
	fmt.Println("\nDry Run Results - Records that would be deleted:")
	fmt.Printf("%-10s %-10s %-10s %-12s\n", "JournalID", "DetailID", "HeaderID", "TxnDate")
	fmt.Println(strings.Repeat("-", 50))

	count := 0
	for _, record := range records {
		if count >= 20 { // Show only first 20 records
			fmt.Printf("... and %d more records\n", len(records)-20)
			break
		}
		fmt.Printf("%-10d %-10d %-10d %-12s\n",
			record.JournalID, record.DetailID, record.HeaderID, record.TxnDate)
		count++
	}
}

// ShowOrphanedHeaders displays orphaned headers that would be deleted
func ShowOrphanedHeaders(headers []OrphanedHeader) {
	fmt.Println("\nOrphaned headers that would be deleted:")
	fmt.Printf("%-10s %-20s %-12s %-10s %-10s\n", "ID", "HeaderNo", "FormDate", "PartnerFk", "FormType")
	fmt.Println(strings.Repeat("-", 72))

	count := 0
	for _, header := range headers {
		if count >= 20 {
			fmt.Printf("... and %d more records\n", len(headers)-20)
			break
		}
		fmt.Printf("%-10d %-20s %-12s %-10d %-10d\n",
			header.ID, header.HeaderNo, header.FormDate, header.PartnerFk, header.FormType)
		count++
	}
}
