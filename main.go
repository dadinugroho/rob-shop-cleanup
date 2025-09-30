package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Error loading configuration:", err)
	}

	// Setup logging
	logger, logFile, err := SetupLogger(config.LogFile)
	if err != nil {
		log.Fatal("Failed to setup logger:", err)
	}
	defer logFile.Close()

	logger.Printf("Starting cleanup process with cutoff date: %s, DryRun: %v", config.CutoffDate, config.DryRun)

	// Connect to database
	db, err := ConnectDatabase(config)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	fmt.Printf("Connected to database successfully\n")
	fmt.Printf("Cutoff date: %s\n", config.CutoffDate)
	fmt.Printf("Dry run mode: %v\n", config.DryRun)
	fmt.Printf("Log file: %s\n", config.LogFile)

	// Parse cutoff date
	cutoffDate, err := time.Parse("2006-01-02", config.CutoffDate)
	if err != nil {
		log.Fatal("Invalid cutoff date format. Use YYYY-MM-DD:", err)
	}

	// Create cleanup service
	cleanupService := NewCleanupService(db, logger)

	// ============================================================
	// STEP 1: Clean up zero-balance items (journal + form_detail)
	// ============================================================
	fmt.Println("\n=== STEP 1: Cleaning up zero-balance items ===")
	logger.Println("STEP 1: Starting zero-balance cleanup")

	// Find items that had zero balance on or before cutoff date
	zeroBalanceItems, err := cleanupService.FindZeroBalanceItemsByDate(cutoffDate)
	if err != nil {
		log.Fatal("Error finding zero balance items:", err)
	}

	fmt.Printf("Found %d item locations with zero balance on or before %s\n", len(zeroBalanceItems), config.CutoffDate)
	logger.Printf("Found %d item locations with zero balance", len(zeroBalanceItems))

	if len(zeroBalanceItems) > 0 {
		// Get records to delete
		recordsToDelete, err := cleanupService.GetRecordsToDelete(zeroBalanceItems, cutoffDate)
		if err != nil {
			log.Fatal("Error identifying records to delete:", err)
		}

		// Show statistics
		stats := CalculateStats(recordsToDelete)
		fmt.Printf("Records that will be processed:\n")
		fmt.Printf("  Journal records: %d\n", len(recordsToDelete))
		fmt.Printf("  Form detail records: %d\n", stats.DetailRecords)

		logger.Printf("Records to process: %d journal, %d detail", 
			len(recordsToDelete), stats.DetailRecords)

		if config.DryRun {
			fmt.Println("\n=== DRY RUN MODE - No actual deletion will occur ===")
			ShowDryRunResults(recordsToDelete)
			logger.Println("Dry run for zero-balance items completed")
		} else {
			// Perform deletion
			err = cleanupService.PerformDeletion(recordsToDelete)
			if err != nil {
				log.Fatal("Error performing deletion:", err)
			}

			fmt.Println("Zero-balance items cleanup completed successfully!")
			logger.Printf("Zero-balance cleanup completed. Deleted %d journal and %d detail records", 
				len(recordsToDelete), stats.DetailRecords)
		}
	} else {
		fmt.Println("No items with zero balance found.")
		logger.Println("No zero-balance items found for cleanup")
	}

	// ============================================================
	// STEP 2: Clean up orphaned headers
	// ============================================================
	fmt.Println("\n=== STEP 2: Cleaning up orphaned headers ===")
	logger.Println("STEP 2: Starting orphaned headers cleanup")

	orphanedHeaders, err := cleanupService.FindOrphanedHeaders()
	if err != nil {
		log.Fatal("Error finding orphaned headers:", err)
	}

	fmt.Printf("Found %d orphaned form_header records (no associated form_detail)\n", len(orphanedHeaders))
	logger.Printf("Found %d orphaned headers", len(orphanedHeaders))

	if len(orphanedHeaders) > 0 {
		if config.DryRun {
			fmt.Println("\n=== DRY RUN MODE - No actual deletion will occur ===")
			ShowOrphanedHeaders(orphanedHeaders)
			logger.Println("Dry run for orphaned headers completed")
		} else {
			err = cleanupService.DeleteOrphanedHeaders(orphanedHeaders)
			if err != nil {
				log.Fatal("Error deleting orphaned headers:", err)
			}

			fmt.Println("Orphaned headers cleanup completed successfully!")
			logger.Printf("Orphaned headers cleanup completed. Deleted %d headers", len(orphanedHeaders))
		}
	} else {
		fmt.Println("No orphaned headers found.")
		logger.Println("No orphaned headers found")
	}

	// ============================================================
	// STEP 3: Show summary
	// ============================================================
	fmt.Println("\n=== STEP 3: Summary ===")
	err = cleanupService.ShowRemainingBalance()
	if err != nil {
		log.Fatal("Error showing remaining balance:", err)
	}

	fmt.Println("\n✅ All cleanup operations completed successfully!")
	logger.Println("All cleanup operations completed successfully")
}
