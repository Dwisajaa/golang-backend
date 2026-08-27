package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const technicianProfileColumns = "id, user_id, technician_code, phone, specialization, address, bio, is_active, created_at, updated_at"

// MySQLTechnicianProfileStore implements TechnicianProfileStore.
type MySQLTechnicianProfileStore struct{}

func NewMySQLTechnicianProfileStore() *MySQLTechnicianProfileStore {
	return &MySQLTechnicianProfileStore{}
}

func (s *MySQLTechnicianProfileStore) FindByUserID(ctx context.Context, q Queryer, userID uint64) (*model.TechnicianProfile, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+technicianProfileColumns+" FROM technician_profiles WHERE user_id = ?", userID)

	p := &model.TechnicianProfile{}
	var phone, specialization, address, bio sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(
		&p.ID, &p.UserID, &p.TechnicianCode,
		&phone, &specialization, &address, &bio, &p.IsActive,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Phone = nullStringPtr(phone)
	p.Specialization = nullStringPtr(specialization)
	p.Address = nullStringPtr(address)
	p.Bio = nullStringPtr(bio)
	p.CreatedAt = nullTimePtr(createdAt)
	p.UpdatedAt = nullTimePtr(updatedAt)
	return p, nil
}

func (s *MySQLTechnicianProfileStore) InsertProfile(ctx context.Context, q Queryer, p *model.TechnicianProfile) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO technician_profiles
		   (user_id, technician_code, phone, specialization, address, bio,
		    is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		p.UserID, p.TechnicianCode, p.Phone, p.Specialization, p.Address, p.Bio,
		p.CreatedAt, p.UpdatedAt)
	if err != nil {
		if key, dup := duplicateTarget(err); dup {
			if strings.Contains(key, "technician_code") {
				return ErrDuplicateTechnicianCode
			}
			return ErrDuplicate
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = uint64(id)
	p.IsActive = true
	return nil
}

func (s *MySQLTechnicianProfileStore) UpdateProfile(ctx context.Context, q Queryer, userID uint64, p *model.TechnicianProfile) error {
	_, err := q.ExecContext(ctx,
		`UPDATE technician_profiles
		 SET phone = ?, specialization = ?, address = ?, bio = ?, updated_at = ?
		 WHERE user_id = ?`,
		p.Phone, p.Specialization, p.Address, p.Bio, p.UpdatedAt, userID)
	return err
}
