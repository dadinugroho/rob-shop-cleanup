package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type CleanupService struct {
	db     *sql.DB
	logger *log.Logger
}

func NewCleanupService(db *sql.DB, logger *log.Logger) *CleanupService {
	return &CleanupService{
		db:     db,
		logger: logger,
	}
}

func (cs *CleanupService) FindZeroBalanceItemsByDate(cutoffDate time.Time) ([]ItemBalance, error) {
	query := `
		SELECT 
			j.referenceFk,
			j.itemFk,
			j.locationFk,
			j.shopFk,
			SUM(CASE WHEN j.type > 0 THEN j.quantity ELSE 0 END) as total_purchases,
			SUM(CASE WHEN j.type < 0 THEN j.quantity ELSE 0 END) as total_sales,
			SUM(j.type * j.quantity) as net_balance,
			MAX(j.journalDate) as last_txn_date
		FROM journal j
		WHERE j.accountFk = 2
		  AND j.journalDate <= ?
		  AND j.referenceFk IS NOT NULL
		GROUP BY j.referenceFk, j.itemFk, j.locationFk, j.shopFk
		HAVING SUM(j.type * j.quantity) = 0
		ORDER BY j.referenceFk
	`

	rows, err := cs.db.Query(query, cutoffDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ItemBalance
	for rows.Next() {
		var item ItemBalance
		var lastTxnDate sql.NullString
		
		err := rows.Scan(
			&item.ReferenceFk,
			&item.ItemFk,
			&item.LocationFk,
			&item.ShopFk,
			&item.TotalPurchases,
			&item.TotalSales,
			&item.NetBalance,
			&lastTxnDate,
		)
		if err != nil {
			return nil, err
		}
		
		if lastTxnDate.Valid {
			item.LastTxnDate = lastTxnDate.String
		}
		
		items = append(items, item)
	}

	return items, rows.Err()
}

func (cs *CleanupService) GetRecordsToDelete(items []ItemBalance, cutoffDate time.Time) ([]DeletedRecord, error) {
	if len(items) == 0 {
		return []DeletedRecord{}, nil
	}

	var referenceFks []int
	for _, item := range items {
		referenceFks = append(referenceFks, item.ReferenceFk)
	}

	inClause := ""
	for i, refFk := range referenceFks {
		if i > 0 {
			inClause += ","
		}
		inClause += fmt.Sprintf("%d", refFk)
	}

	query := fmt.Sprintf(`
		SELECT 
			j.id as journal_id,
			j.detailFk as detail_id,
			fd.headerFk as header_id,
			j.journalDate as txn_date
		FROM journal j
		INNER JOIN form_detail fd ON j.detailFk = fd.id
		INNER JOIN form_header fh ON fd.headerFk = fh.id
		WHERE j.accountFk = 2
		  AND j.referenceFk IN (%s)
		  AND j.journalDate <= ?
		ORDER BY j.referenceFk, j.journalDate
	`, inClause)

	rows, err := cs.db.Query(query, cutoffDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeletedRecord
	for rows.Next() {
		var record DeletedRecord
		err := rows.Scan(
			&record.JournalID,
			&record.DetailID,
			&record.HeaderID,
			&record.TxnDate,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func (cs *CleanupService) PerformDeletion(records []DeletedRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	detailQuantities := make(map[int]float64)
	journalIDsByDetail := make(map[int][]int)
	
	for _, record := range records {
		var quantity float64
		var recType int
		err := tx.QueryRow("SELECT quantity, type FROM journal WHERE id = ?", record.JournalID).Scan(&quantity, &recType)
		if err != nil {
			cs.logger.Printf("Error getting journal quantity for ID %d: %v", record.JournalID, err)
			return err
		}
		
		actualQuantity := float64(recType) * quantity
		detailQuantities[record.DetailID] += actualQuantity
		journalIDsByDetail[record.DetailID] = append(journalIDsByDetail[record.DetailID], record.JournalID)
	}

	var allJournalIDs []int
	for _, ids := range journalIDsByDetail {
		allJournalIDs = append(allJournalIDs, ids...)
	}
	
	fmt.Printf("Deleting %d journal records...\n", len(allJournalIDs))
	cs.logger.Printf("Deleting %d journal records: %v", len(allJournalIDs), allJournalIDs)
	
	if len(allJournalIDs) > 0 {
		err = deleteByIDs(tx, "journal", allJournalIDs)
		if err != nil {
			cs.logger.Printf("Error deleting journal records: %v", err)
			return err
		}
	}

	detailsToDelete := []int{}
	detailsUpdated := 0
	
	for detailID, reducedQty := range detailQuantities {
		var currentQty float64
		err := tx.QueryRow("SELECT quantity FROM form_detail WHERE id = ?", detailID).Scan(&currentQty)
		if err != nil {
			cs.logger.Printf("Error getting form_detail quantity for ID %d: %v", detailID, err)
			return err
		}
		
		newQty := currentQty - reducedQty
		
		cs.logger.Printf("form_detail ID %d: current_qty=%.3f, reduced_by=%.3f, new_qty=%.3f", 
			detailID, currentQty, reducedQty, newQty)
		
		if newQty <= 0.001 {
			detailsToDelete = append(detailsToDelete, detailID)
			cs.logger.Printf("form_detail ID %d will be deleted (qty would be %.3f)", detailID, newQty)
		} else {
			_, err := tx.Exec("UPDATE form_detail SET quantity = ? WHERE id = ?", newQty, detailID)
			if err != nil {
				cs.logger.Printf("Error updating form_detail ID %d: %v", detailID, err)
				return err
			}
			detailsUpdated++
			cs.logger.Printf("form_detail ID %d updated: new quantity = %.3f", detailID, newQty)
		}
	}

	if len(detailsToDelete) > 0 {
		fmt.Printf("Deleting %d form_detail records with zero quantity...\n", len(detailsToDelete))
		cs.logger.Printf("Deleting %d form_detail records: %v", len(detailsToDelete), detailsToDelete)
		
		err = deleteByIDs(tx, "form_detail", detailsToDelete)
		if err != nil {
			cs.logger.Printf("Error deleting form_detail records: %v", err)
			return err
		}
	}
	
	if detailsUpdated > 0 {
		fmt.Printf("Updated %d form_detail records with reduced quantities\n", detailsUpdated)
		cs.logger.Printf("Updated %d form_detail records with reduced quantities", detailsUpdated)
	}

	err = tx.Commit()
	if err != nil {
		cs.logger.Printf("Error committing transaction: %v", err)
		return err
	}

	return nil
}

func (cs *CleanupService) ShowRemainingBalance() error {
	query := `
		SELECT 
			referenceFk,
			itemFk,
			locationFk,
			shopFk,
			SUM(CASE WHEN type > 0 THEN quantity ELSE 0 END) as total_purchases,
			SUM(CASE WHEN type < 0 THEN quantity ELSE 0 END) as total_sales,
			SUM(type * quantity) as net_balance,
			COUNT(*) as transaction_count
		FROM journal
		WHERE accountFk = 2 
		  AND referenceFk IS NOT NULL
		GROUP BY referenceFk, itemFk, locationFk, shopFk
		HAVING SUM(type * quantity) > 0
		ORDER BY referenceFk
		LIMIT 10
	`

	rows, err := cs.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("\nRemaining items with positive balance (first 10):")
	fmt.Printf("%-10s %-8s %-10s %-8s %-12s %-12s %-12s %-8s\n", 
		"RefFk", "ItemFk", "LocationFk", "ShopFk", "Purchases", "Sales", "Balance", "TxnCount")
	fmt.Println(strings.Repeat("-", 90))

	count := 0
	for rows.Next() {
		var referenceFk, itemFk, locationFk, shopFk, txnCount int
		var totalPurchases, totalSales, netBalance float64
		
		err := rows.Scan(
			&referenceFk, &itemFk, &locationFk, &shopFk,
			&totalPurchases, &totalSales, &netBalance, &txnCount,
		)
		if err != nil {
			return err
		}

		fmt.Printf("%-10d %-8d %-10d %-8d %-12.3f %-12.3f %-12.3f %-8d\n",
			referenceFk, itemFk, locationFk, shopFk,
			totalPurchases, totalSales, netBalance, txnCount)
		count++
	}

	if count == 0 {
		fmt.Println("No items with positive balance found.")
	}

	var totalItems int
	err = cs.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT referenceFk
			FROM journal
			WHERE accountFk = 2 
			  AND referenceFk IS NOT NULL
			GROUP BY referenceFk, itemFk, locationFk, shopFk
			HAVING SUM(type * quantity) > 0
		) as temp
	`).Scan(&totalItems)
	
	if err == nil {
		fmt.Printf("\nTotal reference items with positive balance: %d\n", totalItems)
	}

	return rows.Err()
}

func (cs *CleanupService) FindOrphanedHeaders() ([]OrphanedHeader, error) {
	query := `
		SELECT 
			fh.id,
			fh.headerNo,
			fh.formDate,
			fh.partnerFk,
			fh.formType
		FROM form_header fh
		LEFT JOIN form_detail fd ON fh.id = fd.headerFk
		WHERE fd.id IS NULL
		ORDER BY fh.id
	`

	rows, err := cs.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var headers []OrphanedHeader
	for rows.Next() {
		var header OrphanedHeader
		err := rows.Scan(
			&header.ID,
			&header.HeaderNo,
			&header.FormDate,
			&header.PartnerFk,
			&header.FormType,
		)
		if err != nil {
			return nil, err
		}
		headers = append(headers, header)
	}

	return headers, rows.Err()
}

func (cs *CleanupService) DeleteOrphanedHeaders(headers []OrphanedHeader) error {
	if len(headers) == 0 {
		return nil
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var headerIDs []int
	for _, header := range headers {
		headerIDs = append(headerIDs, header.ID)
	}

	fmt.Printf("Deleting %d orphaned form_header records...\n", len(headerIDs))
	cs.logger.Printf("Deleting %d orphaned form_header records: %v", len(headerIDs), headerIDs)

	err = deleteByIDs(tx, "form_header", headerIDs)
	if err != nil {
		cs.logger.Printf("Error deleting orphaned headers: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		cs.logger.Printf("Error committing transaction: %v", err)
		return err
	}

	return nil
}

func (cs *CleanupService) FindZeroQuantityDetails() ([]int, error) {
	query := `
		SELECT id 
		FROM form_detail 
		WHERE quantity <= 0.001
		ORDER BY id
	`

	rows, err := cs.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func (cs *CleanupService) DeleteZeroQuantityDetails(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	fmt.Printf("Deleting %d zero-quantity form_detail records...\n", len(ids))
	cs.logger.Printf("Deleting %d zero-quantity form_detail records: %v", len(ids), ids)

	err = deleteByIDs(tx, "form_detail", ids)
	if err != nil {
		cs.logger.Printf("Error deleting zero-quantity form_detail: %v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		cs.logger.Printf("Error committing transaction: %v", err)
		return err
	}

	return nil
}

func buildWhereClause(items []ItemBalance) string {
	if len(items) == 0 {
		return "1=0"
	}

	whereClause := "("
	for i, item := range items {
		if i > 0 {
			whereClause += " OR "
		}
		whereClause += fmt.Sprintf(
			"(j.referenceFk = %d)",
			item.ReferenceFk,
		)
	}
	whereClause += ")"
	return whereClause
}

func deleteByIDs(tx *sql.Tx, tableName string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	batchSize := 1000
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := ids[i:end]
		inClause := ""
		for j, id := range batch {
			if j > 0 {
				inClause += ","
			}
			inClause += fmt.Sprintf("%d", id)
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", tableName, inClause)
		_, err := tx.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}
