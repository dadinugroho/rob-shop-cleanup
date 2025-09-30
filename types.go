package main

// ItemBalance represents an item's balance information
type ItemBalance struct {
	ReferenceFk    int
	ItemFk         int
	LocationFk     int
	ShopFk         int
	TotalPurchases float64
	TotalSales     float64
	NetBalance     float64
	LastTxnDate    string
}

// DeletedRecord represents a record that will be deleted
type DeletedRecord struct {
	JournalID int
	DetailID  int
	HeaderID  int
	TxnDate   string
}

// Stats represents statistics about records to be processed
type Stats struct {
	JournalRecords int
	DetailRecords  int
	HeaderRecords  int
}

// OrphanedHeader represents a form_header with no form_detail
type OrphanedHeader struct {
	ID        int
	HeaderNo  string
	FormDate  string
	PartnerFk int
	FormType  int
}
