package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/lib/pq"

	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/api"
)

type DB struct {
	Pool *pgxpool.Pool
}

var (
	ErrForbidden      = errors.New("forbidden")
	ErrUserNotFound   = errors.New("user not found")
	ErrTenderNotFound = errors.New("tender not found")
	ErrBidNotFound    = errors.New("bid not found")
)

func NewDB(ctx context.Context, conn string) (*DB, error) {
	time.Sleep(time.Second)
	pool, err := pgxpool.Connect(ctx, conn)
	if err != nil {
		return nil, err
	}
	return &DB{
		Pool: pool,
	}, nil
}

func (db *DB) GetTenders(filters api.GetTendersParams) ([]api.Tender, error) {
	var tenders []api.Tender
	var queryBuilder strings.Builder
	var args []interface{}
	var argCount int

	queryBuilder.WriteString(`
        SELECT id, name, description, organization_id, service_type, status, version, created_at
        FROM tenders
        WHERE 1=1
    `)

	if filters.ServiceType != nil && len(*filters.ServiceType) > 0 {
		argCount++
		queryBuilder.WriteString(fmt.Sprintf(" AND service_type = ANY($%d)", argCount))
		args = append(args, *filters.ServiceType)
	}

	if filters.Limit != nil {
		argCount++
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argCount))
		args = append(args, *filters.Limit)
	}

	if filters.Offset != nil {
		argCount++
		queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", argCount))
		args = append(args, *filters.Offset)
	}

	query := queryBuilder.String()

	log.Printf("Executing query to get tenders: %s with args: %v", query, args)

	rows, err := db.Pool.Query(context.Background(), query, args...)
	if err != nil {
		log.Printf("Error executing query to get tenders: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t api.Tender
		var createdAt time.Time

		err := rows.Scan(
			&t.Id,
			&t.Name,
			&t.Description,
			&t.OrganizationId,
			&t.ServiceType,
			&t.Status,
			&t.Version,
			&createdAt,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		t.CreatedAt = createdAt.Format(time.RFC3339)
		tenders = append(tenders, t)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after processing rows: %v", err)
		return nil, err
	}

	log.Printf("Successfully retrieved %d tenders", len(tenders))

	return tenders, nil
}

// Создание нового тендера
func (db *DB) CreateTender(tender api.Tender, creatorUsername string) (api.Tender, error) {
	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return api.Tender{}, fmt.Errorf("could not start transaction: %v", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil {
				log.Printf("Error rolling back transaction: %v", rollbackErr)
			}
		}
	}()

	var employeeExists bool
	checkEmployeeQuery := `SELECT EXISTS(SELECT 1 FROM employee WHERE username = $1)`
	err = tx.QueryRow(context.Background(), checkEmployeeQuery, creatorUsername).Scan(&employeeExists)
	if err != nil {
		log.Printf("Error checking employee existence: %v", err)
		return api.Tender{}, fmt.Errorf("could not check employee existence: %v", err)
	}
	if !employeeExists {
		return api.Tender{}, fmt.Errorf("employee does not exist")
	}

	var organizationExists bool
	checkOrganizationQuery := `SELECT EXISTS(SELECT 1 FROM organization WHERE id = $1)`
	err = tx.QueryRow(context.Background(), checkOrganizationQuery, tender.OrganizationId).Scan(&organizationExists)
	if err != nil {
		log.Printf("Error checking organization existence: %v", err)
		return api.Tender{}, fmt.Errorf("could not check organization existence: %v", err)
	}
	if !organizationExists {
		return api.Tender{}, fmt.Errorf("organization does not exist")
	}

	query := `
        INSERT INTO tenders (name, description, organization_id, service_type, status, version, creator_username)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, name, description, organization_id, service_type, status, version, created_at
    `

	var createdTender api.Tender
	var createdAt time.Time

	err = tx.QueryRow(context.Background(), query,
		tender.Name,
		tender.Description,
		tender.OrganizationId,
		tender.ServiceType,
		tender.Status,
		tender.Version,
		creatorUsername,
	).Scan(
		&createdTender.Id,
		&createdTender.Name,
		&createdTender.Description,
		&createdTender.OrganizationId,
		&createdTender.ServiceType,
		&createdTender.Status,
		&createdTender.Version,
		&createdAt,
	)

	if err != nil {
		log.Printf("Error creating tender: %v", err)
		return api.Tender{}, ErrForbidden
	}

	if err := tx.Commit(context.Background()); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return api.Tender{}, fmt.Errorf("could not commit transaction: %v", err)
	}

	createdTender.CreatedAt = createdAt.Format(time.RFC3339)

	log.Printf("Tender created successfully: %v", createdTender)
	return createdTender, nil
}

func (db *DB) GetUserTenders(username string, limit int32, offset int32) ([]api.Tender, error) {
	var tenders []api.Tender

	query := `
        SELECT id, name, description, organization_id, service_type, status, version, created_at
        FROM tenders
        WHERE organization_id = (
            SELECT organization_id
            FROM employee
            WHERE username = $1
        )
        ORDER BY created_at DESC
    `

	if limit > 0 && offset >= 0 {
		query += " LIMIT $2 OFFSET $3"
	}

	var rows pgx.Rows
	var err error
	if limit > 0 && offset >= 0 {
		rows, err = db.Pool.Query(context.Background(), query, username, limit, offset)
	} else {
		rows, err = db.Pool.Query(context.Background(), query, username)
	}

	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t api.Tender
		var createdAt time.Time

		err := rows.Scan(
			&t.Id,
			&t.Name,
			&t.Description,
			&t.OrganizationId,
			&t.ServiceType,
			&t.Status,
			&t.Version,
			&createdAt,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		t.CreatedAt = createdAt.Format(time.RFC3339)
		tenders = append(tenders, t)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after processing rows: %v", err)
		return nil, err
	}

	log.Printf("Successfully retrieved %d tenders for user %s", len(tenders), username)

	return tenders, nil
}

func (db *DB) EditTender(tenderId string, name string, description string, serviceType string, creatorUsername string) (api.Tender, error) {
	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return api.Tender{}, err
	}
	defer tx.Rollback(context.Background())

	var updatedTender api.Tender
	var createdAt time.Time

	log.Printf("Checking permission for user %s to edit tender %s", creatorUsername, tenderId)
	hasPermission, err := db.CheckUserTenderPermission(tenderId, creatorUsername, "edit")
	if err != nil {
		log.Printf("Error checking permission for user %s on tender %s: %v", creatorUsername, tenderId, err)
		return api.Tender{}, err
	}
	if !hasPermission {
		log.Printf("User %s does not have permission to view tender %s", creatorUsername, tenderId)
		return api.Tender{}, ErrForbidden
	}

	query := `
        UPDATE tenders
        SET name = $1, description = $2, service_type = $3, version = version + 1
        WHERE id = $4
        RETURNING id, name, description, organization_id, service_type, status, version, created_at
    `

	log.Printf("Editing tender: id=%s, name=%s, description=%s, serviceType=%s", tenderId, name, description, serviceType)

	err = db.Pool.QueryRow(context.Background(), query, name, description, serviceType, tenderId).Scan(
		&updatedTender.Id,
		&updatedTender.Name,
		&updatedTender.Description,
		&updatedTender.OrganizationId,
		&updatedTender.ServiceType,
		&updatedTender.Status,
		&updatedTender.Version,
		&createdAt,
	)
	if err != nil {
		log.Printf("Error updating tender with id=%s: %v", tenderId, err)
		return api.Tender{}, err
	}

	updatedTender.CreatedAt = createdAt.Format(time.RFC3339)

	err = tx.Commit(context.Background())
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		return api.Tender{}, err
	}

	log.Printf("Successfully updated tender with id=%s", updatedTender.Id)

	return updatedTender, nil
}

func (db *DB) RollbackTender(tenderId string, version int, username string) (api.Tender, error) {
	log.Printf("Rolling back tender %s to version %d by user %s", tenderId, version, username)

	var updatedTender api.Tender
	var existingTender api.Tender
	var createdAt time.Time

	query := `
        SELECT id, name, description, service_type, status, version, created_at
        FROM tenders
        WHERE id = $1 AND version = $2
    `
	err := db.Pool.QueryRow(context.Background(), query, tenderId, version).Scan(
		&existingTender.Id,
		&existingTender.Name,
		&existingTender.Description,
		&existingTender.ServiceType,
		&existingTender.Status,
		&existingTender.Version,
		&createdAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("No tender found with id %s and version %d", tenderId, version)
			return api.Tender{}, ErrTenderNotFound
		}
		log.Printf("Error retrieving tender %s with version %d: %v", tenderId, version, err)
		return api.Tender{}, err
	}

	log.Printf("Tender found for rollback: %v", existingTender)

	query = `
        UPDATE tenders
        SET name = $1, description = $2, service_type = $3, version = version + 1
        WHERE id = $4
        RETURNING id, name, description, service_type, status, version, created_at
    `
	err = db.Pool.QueryRow(context.Background(), query, existingTender.Name, existingTender.Description, existingTender.ServiceType, tenderId).Scan(
		&updatedTender.Id,
		&updatedTender.Name,
		&updatedTender.Description,
		&updatedTender.ServiceType,
		&updatedTender.Status,
		&updatedTender.Version,
		&createdAt,
	)
	if err != nil {
		log.Printf("Error updating tender %s during rollback: %v", tenderId, err)
		return api.Tender{}, err
	}

	updatedTender.CreatedAt = createdAt.Format(time.RFC3339)

	log.Printf("Successfully rolled back tender %s to version %d", tenderId, updatedTender.Version)
	return updatedTender, nil
}

func (db *DB) GetTenderStatus(tenderId string, username string) (string, error) {
	log.Printf("Checking permission for user %s to view tender %s", username, tenderId)
	hasPermission, err := db.CheckUserTenderPermission(tenderId, username, "edit")
	if err != nil {
		log.Printf("Error checking permission for user %s on tender %s: %v", username, tenderId, err)
		return "", err
	}
	if !hasPermission {
		log.Printf("User %s does not have permission to view tender %s", username, tenderId)
		return "", ErrForbidden
	}

	var status string
	query := `
        SELECT status
        FROM tenders
        WHERE id = $1
    `
	err = db.Pool.QueryRow(context.Background(), query, tenderId).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Tender with id %s not found", tenderId)
			return "", ErrTenderNotFound
		}
		log.Printf("Error retrieving tender status for id %s: %v", tenderId, err)
		return "", err
	}

	log.Printf("Successfully retrieved status for tender %s: %s", tenderId, status)
	return status, nil
}

func (db *DB) UpdateTenderStatus(tenderId string, status api.TenderStatus, username string) (api.Tender, error) {
	log.Printf("Checking permission for user %s to update tender %s", username, tenderId)
	hasPermission, err := db.CheckUserTenderPermission(tenderId, username, "edit")
	if err != nil {
		log.Printf("Error checking permission for user %s on tender %s: %v", username, tenderId, err)
		return api.Tender{}, err
	}
	if !hasPermission {
		log.Printf("User %s does not have permission to update tender %s", username, tenderId)
		return api.Tender{}, ErrForbidden
	}

	updatedStatus := strings.ToUpper(string(status))

	var updatedTender api.Tender
	var createdAt time.Time
	query := `
        UPDATE tenders
        SET status = $1, version = version + 1
        WHERE id = $2
        RETURNING id, name, description, organization_id, service_type, status, version, created_at
    `
	err = db.Pool.QueryRow(context.Background(), query, updatedStatus, tenderId).Scan(
		&updatedTender.Id,
		&updatedTender.Name,
		&updatedTender.Description,
		&updatedTender.OrganizationId,
		&updatedTender.ServiceType,
		&updatedTender.Status,
		&updatedTender.Version,
		&createdAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Tender with id %s not found", tenderId)
			return api.Tender{}, ErrTenderNotFound
		}
		log.Printf("Error updating status for tender %s: %v", tenderId, err)
		return api.Tender{}, err
	}

	updatedTender.CreatedAt = createdAt.Format(time.RFC3339)
	log.Printf("Successfully updated status for tender %s to %s", tenderId, updatedTender.Status)
	return updatedTender, nil
}

func (db *DB) GetUserBids(limit int32, offset int32, username string) ([]api.Bid, error) {
	var bids []api.Bid

	query := `
        SELECT id, name, description, tender_id, author_id, author_type, status, version, created_at
        FROM bids
        WHERE author_id = (
            SELECT id
            FROM employee
            WHERE username = $1
        )
        ORDER BY created_at DESC
    `

	if limit > 0 && offset >= 0 {
		query += " LIMIT $2 OFFSET $3"
	}

	var rows pgx.Rows
	var err error
	if limit > 0 && offset >= 0 {
		rows, err = db.Pool.Query(context.Background(), query, username, limit, offset)
	} else {
		rows, err = db.Pool.Query(context.Background(), query, username)
	}

	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var b api.Bid
		var createdAt time.Time

		err := rows.Scan(
			&b.Id,
			&b.Name,
			&b.Description,
			&b.TenderId,
			&b.AuthorId,
			&b.AuthorType,
			&b.Status,
			&b.Version,
			&createdAt,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		b.CreatedAt = createdAt.Format(time.RFC3339)

		bids = append(bids, b)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after processing rows: %v", err)
		return nil, err
	}

	log.Printf("Successfully retrieved %d bids for user %s", len(bids), username)

	return bids, nil
}

func (db *DB) CreateBid(bid api.Bid) (api.Bid, error) {
	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return api.Bid{}, err
	}
	defer tx.Rollback(context.Background())

	var tenderExists bool
	err = tx.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM tenders WHERE id = $1)
	`, bid.TenderId).Scan(&tenderExists)
	if err != nil {
		log.Printf("Error checking tender existence: %v", err)
		return api.Bid{}, err
	}
	if !tenderExists {
		return api.Bid{}, fmt.Errorf("tender does not exist")
	}

	var authorExists bool
	if bid.AuthorType == "USER" {
		err = tx.QueryRow(context.Background(), `
			SELECT EXISTS(SELECT 1 FROM employee WHERE id = $1)
		`, bid.AuthorId).Scan(&authorExists)
	} else if bid.AuthorType == "ORGANIZATION" {
		err = tx.QueryRow(context.Background(), `
			SELECT EXISTS(SELECT 1 FROM organization WHERE id = $1)
		`, bid.AuthorId).Scan(&authorExists)
	}
	if err != nil {
		log.Printf("Error checking author existence: %v", err)
		return api.Bid{}, err
	}
	if !authorExists {
		return api.Bid{}, fmt.Errorf("author does not exist")
	}

	query := `
        INSERT INTO bids (name, description, tender_id, author_id, author_type, status, version)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, name, description, tender_id, author_id, author_type, status, version, created_at
    `

	var createdBid api.Bid
	var createdAt time.Time

	updatedAuthorType := strings.ToUpper(string(bid.AuthorType))

	err = db.Pool.QueryRow(context.Background(), query,
		bid.Name,
		bid.Description,
		bid.TenderId,
		bid.AuthorId,
		updatedAuthorType,
		bid.Status,
		bid.Version,
	).Scan(
		&createdBid.Id,
		&createdBid.Name,
		&createdBid.Description,
		&createdBid.TenderId,
		&createdBid.AuthorId,
		&createdBid.AuthorType,
		&createdBid.Status,
		&createdBid.Version,
		&createdAt,
	)

	if err != nil {
		log.Printf("Error creating bid: %v", err)
		return api.Bid{}, err
	}

	createdBid.CreatedAt = createdAt.Format(time.RFC3339)

	err = tx.Commit(context.Background())
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		return api.Bid{}, err
	}

	log.Printf("Bid created successfully: %v", createdBid)
	return createdBid, nil
}

func (db *DB) CheckUserTenderPermission(tenderId api.TenderId, username api.Username, action string) (bool, error) {
	// Получаем идентификатор пользователя
	var userId uuid.UUID
	query := `SELECT id FROM employee WHERE username = $1`
	err := db.Pool.QueryRow(context.Background(), query, username).Scan(&userId)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, ErrUserNotFound // Пользователь не найден
		}
		return false, err
	}

	// Получаем статус тендера
	var tenderStatus string
	query = `SELECT status FROM tenders WHERE id = $1`
	err = db.Pool.QueryRow(context.Background(), query, tenderId).Scan(&tenderStatus)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, ErrTenderNotFound // Тендер не найден
		}
		return false, err
	}

	// Определяем права пользователя в зависимости от действия
	switch action {
	case "create":
		// Для создания тендера, пользователь должен быть ответственным за организацию
		var isResponsible bool
		query = `SELECT COUNT(*) > 0 
                 FROM organization_responsible 
                 WHERE user_id = $1 
                 AND organization_id = (SELECT organization_id FROM tenders WHERE id = $2)`
		err = db.Pool.QueryRow(context.Background(), query, userId, tenderId).Scan(&isResponsible)
		if err != nil {
			return false, err
		}
		return isResponsible, nil

	case "publish":
		// Публикация доступна любому авторизованному пользователю
		return true, nil

	case "close":
		// Закрытие тендера доступно только ответственным за организацию
		var isResponsible bool
		query = `SELECT COUNT(*) > 0 
                 FROM organization_responsible 
                 WHERE user_id = $1 
                 AND organization_id = (SELECT organization_id FROM tenders WHERE id = $2)`
		err = db.Pool.QueryRow(context.Background(), query, userId, tenderId).Scan(&isResponsible)
		if err != nil {
			return false, err
		}
		return isResponsible, nil

	case "edit":
		// Редактирование доступно ответственным за организацию или если тендер находится в статусе "PUBLISHED"
		var isResponsible bool
		query = `SELECT COUNT(*) > 0 
                 FROM organization_responsible 
                 WHERE user_id = $1 
                 AND organization_id = (SELECT organization_id FROM tenders WHERE id = $2)`
		err = db.Pool.QueryRow(context.Background(), query, userId, tenderId).Scan(&isResponsible)
		if err != nil {
			return false, err
		}

		if tenderStatus == "PUBLISHED" || isResponsible {
			return true, nil
		}
		return false, nil

	default:
		return false, fmt.Errorf("unknown action: %s", action)
	}
}

func (db *DB) Close() {
	db.Pool.Close()
}
