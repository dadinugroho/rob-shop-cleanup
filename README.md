# Shop Transaction Cleanup Script

A Go-based cleanup utility for managing shop transaction data by removing zero-balance inventory records and orphaned entries.

## Features

- ✅ Cleans up zero-balance journal entries (purchase = sales)
- ✅ Intelligently reduces form_detail quantities instead of deleting
- ✅ Removes zero-quantity form_detail records
- ✅ Cleans up orphaned form_header records
- ✅ Comprehensive logging with all affected IDs
- ✅ Dry-run mode for safe testing
- ✅ Transaction-safe operations

## Requirements

- Go 1.23.5 or higher
- MariaDB/MySQL database
- Database tables: `journal`, `form_detail`, `form_header`

## Installation
```bash
# Clone the repository
git clone https://github.com/dadinugroho/shop-cleanup.git
cd shop-cleanup

# Install dependencies
go mod download
``` 

## Configuration

Copy the example configuration file and edit it:
```bash
cp .env.example .env
```
Then edit .env with your database credentials:
```bash
DB_HOST=localhost
DB_PORT=3306
DB_USER=your_username
DB_PASSWORD=your_password
DB_NAME=your_database
CUTOFF_DATE=2025-01-01
DRY_RUN=true
```

### Configuration Options
```bash
DB_HOST: Database host (default: localhost)
DB_PORT: Database port (default: 3306)
DB_USER: Database username (required)
DB_PASSWORD: Database password (required)
DB_NAME: Database name (required)
CUTOFF_DATE: Only process records on or before this date (format: YYYY-MM-DD)
DRY_RUN: Set to true for preview mode, false for actual cleanup
```
Usage
```bash
Test Run (Dry Run)
bashDRY_RUN=true go run .
```

Actual Cleanup
```bash
bashDRY_RUN=false go run .
```

## How It Works
### Step 1: Zero-Balance Cleanup

Finds journal entries where SUM(type * quantity) = 0 per referenceFk
Deletes journal entries
Reduces quantity in form_detail (doesn't delete immediately)
Only deletes form_detail when quantity becomes 0

### Step 2: Zero-Quantity Details

Finds and removes form_detail records with quantity = 0

### Step 3: Orphaned Headers

Finds form_header records with no associated form_detail
Removes orphaned headers

### Step 4: Summary

Shows remaining items with positive balance

### Output Example
Connected to database successfully
Cutoff date: 2025-01-01
Dry run mode: true
Log file: cleanup_20250929_154355.log

=== STEP 1: Cleaning up zero-balance items ===
Found 150 item locations with zero balance
Deleting 450 journal records...
form_detail ID 50: current_qty=5.000, reduced_by=2.000, new_qty=3.000
form_detail ID 51: current_qty=2.000, reduced_by=2.000, new_qty=0.000
Updated 1 form_detail records with reduced quantities

=== STEP 2: Cleaning up zero-quantity form_detail ===
Found 5 form_detail records with quantity = 0
Deleted 5 zero-quantity form_detail records

=== STEP 3: Cleaning up orphaned headers ===
Found 10 orphaned form_header records
Deleted 10 orphaned form_header records

✅ All cleanup operations completed successfully!


## Logging
All operations are logged to timestamped log files: cleanup_YYYYMMDD_HHMMSS.log
Log files include:

All deleted journal IDs
All affected form_detail IDs with quantity changes
All deleted form_header IDs

## Project Structure
```
rob-shop-cleanup/
├── main.go              # Entry point and workflow orchestration
├── config.go           # Configuration management
├── database.go         # Database connection
├── logger.go           # Logging setup
├── types.go            # Data structures
├── cleanup_service.go  # Main business logic
├── utils.go            # Utility functions
├── .env                # Configuration file (not in git)
├── .env.example        # Example configuration
├── go.mod              # Go module
└── README.md           # This file
```

## Safety Features

Dry-run mode: Test before executing
Transaction safety: All operations use database transactions
Comprehensive logging: Track every change
Cutoff date: Only process old data
Quantity reduction: Preserves partial records

## Database Schema
This script expects the following tables:

### journal
id: Primary key
accountFk: Account type (script filters for accountFk = 2)
referenceFk: Reference to form_detail.id
itemFk: Item identifier
locationFk: Location identifier
shopFk: Shop identifier
type: Transaction type (+1 incoming, -1 outgoing)
quantity: Transaction quantity (always positive)
journalDate: Transaction date

### form_detail
id: Primary key
headerFk: Foreign key to form_header
quantity: Current quantity

### form_header
id: Primary key
headerNo: Header number
formDate: Form date
partnerFk: Partner identifier
formType: Form type

## License
MIT License
Author
Daniel Adinugroho
adinugro@gmail.com
